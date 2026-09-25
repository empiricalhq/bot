package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"whatsbot/internal/config"
)

// TestFindEnvFile tests the findEnvFile function.
func TestFindEnvFile(t *testing.T) {
	// Create a temporary directory structure with .git to mark repo root
	tmpRoot := t.TempDir()

	// Create .git directory to mark this as repository root
	gitDir := filepath.Join(tmpRoot, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	subdir := filepath.Join(tmpRoot, "subdir")
	subsubdir := filepath.Join(subdir, "subsubdir")

	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.Mkdir(subsubdir, 0o755); err != nil {
		t.Fatalf("Failed to create subsubdir: %v", err)
	}

	// Create .env in root
	envPath := filepath.Join(tmpRoot, ".env")
	if err := os.WriteFile(envPath, []byte("TEST=value"), 0o644); err != nil {
		t.Fatalf("Failed to write .env: %v", err)
	}

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	tests := []struct {
		name string
		dir  string
	}{
		{"from root", tmpRoot},
		{"from subdir", subdir},
		{"from subsubdir", subsubdir},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Chdir(tt.dir); err != nil {
				t.Fatalf("Failed to change to %s: %v", tt.dir, err)
			}

			found, err := config.FindEnvFile()
			if err != nil {
				t.Errorf("Expected to find .env, got error: %v", err)
			}

			if found != envPath {
				t.Errorf("Expected path %s, got %s", envPath, found)
			}
		})
	}
}

// TestFindEnvFileNotFound tests that findEnvFile returns error when .env doesn't exist.
func TestFindEnvFileNotFound(t *testing.T) {
	tmpRoot := t.TempDir()

	// Create .git directory to mark this as repository root
	gitDir := filepath.Join(tmpRoot, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpRoot); err != nil {
		t.Fatalf("Failed to change to tmpRoot: %v", err)
	}

	_, err = config.FindEnvFile()
	if err == nil {
		t.Error("Expected error when .env not found, got nil")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got: %v", err)
	}
}

// TestFindEnvFileStopsAtRepoRoot tests that findEnvFile stops at the repository root
// and doesn't traverse beyond it (security feature).
func TestFindEnvFileStopsAtRepoRoot(t *testing.T) {
	// Create outer directory with .env (outside repo)
	outerRoot := t.TempDir()

	outerEnv := filepath.Join(outerRoot, ".env")
	if err := os.WriteFile(outerEnv, []byte("OUTER=true"), 0o644); err != nil {
		t.Fatalf("Failed to write outer .env: %v", err)
	}

	// Create inner "repository" with .git but no .env
	repoRoot := filepath.Join(outerRoot, "repo")
	if err := os.Mkdir(repoRoot, 0o755); err != nil {
		t.Fatalf("Failed to create repo dir: %v", err)
	}

	gitDir := filepath.Join(repoRoot, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}

	subdir := filepath.Join(repoRoot, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Change to subdirectory within the repository
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Failed to change to subdir: %v", err)
	}

	// Should not find the outer .env file (security boundary)
	_, err = config.FindEnvFile()
	if err == nil {
		t.Error("Expected error when .env not found in repo, but found one (security violation)")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got: %v", err)
	}
}

// TestFindEnvFileNoRepoRoot tests that findEnvFile only checks the current
// directory when no repository or module root (.git or go.mod) exists in
// any parent directory, instead of searching all the way to the filesystem
// root (security feature).
func TestFindEnvFileNoRepoRoot(t *testing.T) {
	// Create an outer directory with .env but no .git or go.mod anywhere.
	outerRoot := t.TempDir()

	outerEnv := filepath.Join(outerRoot, ".env")
	if err := os.WriteFile(outerEnv, []byte("OUTER=true"), 0o644); err != nil {
		t.Fatalf("Failed to write outer .env: %v", err)
	}

	subdir := filepath.Join(outerRoot, "subdir")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Failed to change to subdir: %v", err)
	}

	// Without a repository/module root, the search must not climb up to
	// outerRoot and pick up its .env.
	_, err = config.FindEnvFile()
	if err == nil {
		t.Error("Expected error when no repo root exists, but found a .env (security violation)")
	}

	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got: %v", err)
	}

	// The current directory itself should still be checked.
	envPath := filepath.Join(subdir, ".env")
	if err := os.WriteFile(envPath, []byte("LOCAL=true"), 0o644); err != nil {
		t.Fatalf("Failed to write .env: %v", err)
	}

	found, err := config.FindEnvFile()
	if err != nil {
		t.Fatalf("Expected to find .env in current directory, got error: %v", err)
	}

	if found != envPath {
		t.Errorf("Expected path %s, got %s", envPath, found)
	}
}
