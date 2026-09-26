package service_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"whatsbot/internal/service"
)

const validFlow = `{"start_node": "A", "nodes": {"A": {"message": {"type": "text", "content": "hi"}}}}`

func newFlowServer(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(status)

		_, err := w.Write([]byte(body))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return server, &requests
}

func ensureFlow(server *httptest.Server, path string) error {
	logger := slog.New(slog.DiscardHandler)

	err := service.EnsureFlowFile(context.Background(), logger, server.Client(), path, server.URL)
	if err != nil {
		return fmt.Errorf("ensure flow file: %w", err)
	}

	return nil
}

func TestEnsureFlowFileDownloadsWhenMissing(t *testing.T) {
	t.Parallel()

	server, requests := newFlowServer(t, http.StatusOK, validFlow)
	path := filepath.Join(t.TempDir(), "nested", "conversation.json")

	err := ensureFlow(server, path)
	if err != nil {
		t.Fatalf("EnsureFlowFile: %v", err)
	}

	if requests.Load() != 1 {
		t.Errorf("requests = %d, want 1", requests.Load())
	}

	if got := readFile(t, path); got != validFlow {
		t.Errorf("file = %q, want the served flow", got)
	}

	_, err = service.LoadFlow(path)
	if err != nil {
		t.Errorf("downloaded file does not load: %v", err)
	}

	assertOnlyFile(t, filepath.Dir(path), "conversation.json")
}

func TestEnsureFlowFileSkipsNetworkWhenPresent(t *testing.T) {
	t.Parallel()

	server, requests := newFlowServer(t, http.StatusOK, validFlow)
	path := writeFlowFile(t, `{"start_node": "local"}`)

	err := ensureFlow(server, path)
	if err != nil {
		t.Fatalf("EnsureFlowFile: %v", err)
	}

	if requests.Load() != 0 {
		t.Errorf("requests = %d, want 0", requests.Load())
	}
}

func TestEnsureFlowFileDownloadFailures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"not found", http.StatusNotFound, "nope"},
		{"server error", http.StatusInternalServerError, validFlow},
		{"invalid json", http.StatusOK, `{`},
		{"invalid flow", http.StatusOK, `{"start_node": "missing", "nodes": {}}`},
		// A valid flow padded past the size cap: a cut body would still parse.
		{"oversized", http.StatusOK, validFlow + strings.Repeat(" ", 10<<20)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server, _ := newFlowServer(t, testCase.status, testCase.body)
			dir := t.TempDir()
			path := filepath.Join(dir, "conversation.json")

			err := ensureFlow(server, path)
			if err == nil {
				t.Fatal("EnsureFlowFile succeeded, want an error")
			}

			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), server.URL) {
				t.Errorf("error = %v, want it to name %q and %q", err, path, server.URL)
			}

			assertOnlyFile(t, dir)
		})
	}
}

func TestEnsureFlowFileUnreachableServer(t *testing.T) {
	t.Parallel()

	server, _ := newFlowServer(t, http.StatusOK, validFlow)
	server.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "conversation.json")

	err := ensureFlow(server, path)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), server.URL) {
		t.Errorf("error = %v, want one naming %q and %q", err, path, server.URL)
	}

	assertOnlyFile(t, dir)
}

func TestEnsureFlowFileKeepsFileCreatedDuringDownload(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "conversation.json")

	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		err := os.WriteFile(path, []byte("from another process"), 0o600)
		if err != nil {
			t.Errorf("creating the file mid-download: %v", err)
		}

		_, err = resp.Write([]byte(validFlow))
		if err != nil {
			t.Errorf("writing response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	err := ensureFlow(server, path)
	if err != nil {
		t.Fatalf("EnsureFlowFile: %v", err)
	}

	if got := readFile(t, path); got != "from another process" {
		t.Errorf("file = %q, want the file created during the download", got)
	}

	assertOnlyFile(t, dir, "conversation.json")
}

func TestEnsureFlowFileKeepsExistingInvalidFile(t *testing.T) {
	t.Parallel()

	server, requests := newFlowServer(t, http.StatusOK, validFlow)
	path := writeFlowFile(t, `{`)

	err := ensureFlow(server, path)
	if err != nil {
		t.Fatalf("EnsureFlowFile: %v", err)
	}

	if requests.Load() != 0 {
		t.Errorf("requests = %d, want 0", requests.Load())
	}

	if got := readFile(t, path); got != `{` {
		t.Errorf("file = %q, want it untouched", got)
	}

	_, err = service.LoadFlow(path)
	if err == nil || !strings.Contains(err.Error(), "failed to parse flow file") {
		t.Errorf("LoadFlow error = %v, want a parse failure", err)
	}
}

// readFile reads through an os.Root on the file's directory, so the path is never opened as a raw variable.
func readFile(t *testing.T, path string) string {
	t.Helper()

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		err := root.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	data, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}

	return string(data)
}

func assertOnlyFile(t *testing.T, dir string, want ...string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("files in %s = %v, want %v", dir, got, want)
	}
}

func unsupported(oldname, newname string) error {
	return &os.LinkError{Op: "rename", Old: oldname, New: newname, Err: errors.ErrUnsupported}
}

func noLinks(oldname, newname string) error {
	return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: syscall.EPERM}
}

func writeTemp(t *testing.T, dir string) string {
	t.Helper()

	tmp := filepath.Join(dir, ".flow-tmp.json")

	err := os.WriteFile(tmp, []byte(validFlow), 0o600)
	if err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	return tmp
}

func TestPlaceFlowFileLinksWhenRenameUnsupported(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "conversation.json")

	placed, err := service.PlaceFlowFile(unsupported, os.Link, writeTemp(t, dir), path)
	if err != nil {
		t.Fatalf("PlaceFlowFile: %v", err)
	}

	if !placed {
		t.Error("placed = false, want true")
	}

	if got := readFile(t, path); got != validFlow {
		t.Errorf("file = %q, want the downloaded flow", got)
	}
}

func TestPlaceFlowFileKeepsExistingFileWhenRenameUnsupported(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "conversation.json")

	err := os.WriteFile(path, []byte("from another process"), 0o600)
	if err != nil {
		t.Fatalf("writing existing file: %v", err)
	}

	placed, err := service.PlaceFlowFile(unsupported, os.Link, writeTemp(t, dir), path)
	if err != nil {
		t.Fatalf("PlaceFlowFile: %v", err)
	}

	if placed {
		t.Error("placed = true, want false")
	}

	if got := readFile(t, path); got != "from another process" {
		t.Errorf("file = %q, want the existing file", got)
	}
}

func TestPlaceFlowFileFailsWithoutAnySafeMove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "conversation.json")

	_, err := service.PlaceFlowFile(unsupported, noLinks, writeTemp(t, dir), path)
	if err == nil {
		t.Fatal("PlaceFlowFile: want an error when neither move is supported")
	}

	_, statErr := os.Lstat(path)
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("path exists after a failed placement: %v", statErr)
	}
}

func TestEnsureFlowFileSkipsNetworkForDanglingSymlink(t *testing.T) {
	t.Parallel()

	server, requests := newFlowServer(t, http.StatusOK, validFlow)
	path := filepath.Join(t.TempDir(), "conversation.json")

	err := os.Symlink(filepath.Join(t.TempDir(), "missing.json"), path)
	if err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	err = ensureFlow(server, path)
	if err != nil {
		t.Fatalf("EnsureFlowFile: %v", err)
	}

	if n := requests.Load(); n != 0 {
		t.Errorf("requests = %d, want 0", n)
	}
}
