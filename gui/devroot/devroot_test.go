//go:build !dev

package devroot_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNonDevBuildKeepsItsWorkingDirectory(t *testing.T) {
	t.Parallel()

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(workDir) != "devroot" {
		t.Fatalf("working dir = %s, want the package directory", workDir)
	}
}
