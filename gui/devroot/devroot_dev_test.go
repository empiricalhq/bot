//go:build dev

package devroot_test

import (
	"os"
	"path/filepath"
	"testing"

	"whatsbot/internal/config"
)

func TestDevBuildRunsFromRepoRoot(t *testing.T) {
	t.Parallel()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(filepath.Join(workDir, "go.mod"))
	if err != nil {
		t.Fatalf("working dir %s is not the repo root: %v", workDir, err)
	}

	_, err = config.Load()
	if err != nil {
		t.Fatalf("config.Load from the repo root: %v", err)
	}
}
