package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFindEnvFile tests the findEnvFile function
func TestFindEnvFile(t *testing.T) {
	// Create a temporary directory structure
	tmpRoot := t.TempDir()
	subdir := filepath.Join(tmpRoot, "subdir")
	subsubdir := filepath.Join(subdir, "subsubdir")
	
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	if err := os.Mkdir(subsubdir, 0755); err != nil {
		t.Fatalf("Failed to create subsubdir: %v", err)
	}

	// Create .env in root
	envPath := filepath.Join(tmpRoot, ".env")
	if err := os.WriteFile(envPath, []byte("TEST=value"), 0644); err != nil {
		t.Fatalf("Failed to write .env: %v", err)
	}

	// Save original directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Test from root
	if err := os.Chdir(tmpRoot); err != nil {
		t.Fatalf("Failed to change to tmpRoot: %v", err)
	}
	found, err := findEnvFile()
	if err != nil {
		t.Errorf("Expected to find .env from root, got error: %v", err)
	}
	if found != envPath {
		t.Errorf("Expected path %s, got %s", envPath, found)
	}

	// Test from subdir
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Failed to change to subdir: %v", err)
	}
	found, err = findEnvFile()
	if err != nil {
		t.Errorf("Expected to find .env from subdir, got error: %v", err)
	}
	if found != envPath {
		t.Errorf("Expected path %s, got %s", envPath, found)
	}

	// Test from subsubdir
	if err := os.Chdir(subsubdir); err != nil {
		t.Fatalf("Failed to change to subsubdir: %v", err)
	}
	found, err = findEnvFile()
	if err != nil {
		t.Errorf("Expected to find .env from subsubdir, got error: %v", err)
	}
	if found != envPath {
		t.Errorf("Expected path %s, got %s", envPath, found)
	}
}

// TestFindEnvFileNotFound tests that findEnvFile returns error when .env doesn't exist
func TestFindEnvFileNotFound(t *testing.T) {
	tmpRoot := t.TempDir()
	
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpRoot); err != nil {
		t.Fatalf("Failed to change to tmpRoot: %v", err)
	}

	_, err = findEnvFile()
	if err == nil {
		t.Error("Expected error when .env not found, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got: %v", err)
	}
}
