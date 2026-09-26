package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	flowDownloadTimeout = 30 * time.Second
	maxFlowFileBytes    = 10 << 20
	flowDirPerm         = 0o750
)

// EnsureFlowFile makes sure a flow file exists at path, downloading it from url when it is missing.
// An existing entry is left alone, even if invalid or a dangling symlink; LoadFlow reports that. A downloaded file is
// written only if LoadFlow accepts it, via a temp file placed by placeFlowFile, so a failed download leaves nothing
// behind and a file created at path meanwhile is never replaced.
func EnsureFlowFile(ctx context.Context, logger *slog.Logger, client *http.Client, path, url string) error {
	_, err := os.Lstat(path)
	if err == nil {
		return nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to check flow file: %w", err)
	}

	logger.Info("Flow file not found, downloading it", "path", path, "url", url)

	err = downloadFlowFile(ctx, logger, client, path, url)
	if err != nil {
		return fmt.Errorf("flow file %q is missing and downloading it from %q failed: %w", path, url, err)
	}

	return nil
}

func downloadFlowFile(ctx context.Context, logger *slog.Logger, client *http.Client, path, url string) error {
	data, err := fetchFlow(ctx, client, url)
	if err != nil {
		return err
	}

	return writeFlowFile(logger, path, data)
}

// fetchFlow reads the whole body and fails if it is larger than maxFlowFileBytes.
func fetchFlow(ctx context.Context, client *http.Client, url string) (data []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, flowDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			data, err = nil, errors.Join(err, fmt.Errorf("failed to close response: %w", closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	data, err = io.ReadAll(io.LimitReader(resp.Body, maxFlowFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if len(data) > maxFlowFileBytes {
		return nil, fmt.Errorf("response is larger than %d bytes", maxFlowFileBytes)
	}

	return data, nil
}

// writeFlowFile validates data with LoadFlow before it reaches path. It never replaces a file already at path.
func writeFlowFile(logger *slog.Logger, path string, data []byte) (err error) {
	dir := filepath.Dir(path)

	err = os.MkdirAll(dir, flowDirPerm)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".flow-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	// The temp file is always removed; after a rename it is already gone, after a link path holds the flow.
	defer func() {
		err = errors.Join(err, removeIfExists(tmp.Name()))
	}()

	_, err = tmp.Write(data)
	if err != nil {
		return errors.Join(fmt.Errorf("failed to write temp file: %w", err), tmp.Close())
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	_, err = LoadFlow(tmp.Name())
	if err != nil {
		return fmt.Errorf("downloaded file is not a valid flow: %w", err)
	}

	placed, err := placeFlowFile(renameNoReplace, os.Link, tmp.Name(), path)
	if err != nil {
		return fmt.Errorf("failed to move file into place: %w", err)
	}

	if !placed {
		logger.Info("Flow file appeared during download, keeping the existing one", "path", path)
	}

	return nil
}

// placeFlowFile moves tmp to path and reports false if a file was already there. Both moves it tries are atomic
// and refuse an existing file, so a file another process created since the caller's check is never replaced and
// path never holds a partial flow. A filesystem that supports neither fails here.
func placeFlowFile(rename, link func(oldname, newname string) error, tmp, path string) (bool, error) {
	err := rename(tmp, path)
	if errors.Is(err, errors.ErrUnsupported) {
		linkErr := link(tmp, path)
		if linkErr != nil && !errors.Is(linkErr, os.ErrExist) {
			return false, errors.Join(err, linkErr)
		}

		err = linkErr
	}

	if errors.Is(err, os.ErrExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %s: %w", path, err)
	}

	return nil
}
