//go:build dev

package devroot

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// init moves the process to the repo root. `wails dev` must run from gui/,
// but config.Load() reads .env and resolves its relative default paths
// against the working directory.
func init() {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("devroot: cannot locate the repo root")
	}

	err := os.Chdir(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		log.Fatalf("devroot: cannot move to the repo root: %v", err)
	}
}
