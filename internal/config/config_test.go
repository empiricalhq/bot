package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadFromSubdirectory tests that config.Load() can find .env
// even when called from a subdirectory (e.g., gui/).
func TestLoadFromSubdirectory(t *testing.T) {
	// Create a temporary directory structure:
	// tmpRoot/
	//   .env (with test content)
	//   subdir/
	tmpRoot := t.TempDir()
	subdir := filepath.Join(tmpRoot, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create a test .env in the root
	envContent := "LOG_LEVEL=DEBUG\nENV=dev\n"
	envPath := filepath.Join(tmpRoot, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("Failed to write .env: %v", err)
	}

	// Create a test conversation.json file
	flowPath := filepath.Join(tmpRoot, "conversation.json")
	if err := os.WriteFile(flowPath, []byte(`{"nodes": []}`), 0644); err != nil {
		t.Fatalf("Failed to write conversation.json: %v", err)
	}

	// Set environment variables for paths
	os.Setenv("FLOW_FILE_PATH", flowPath)
	os.Setenv("SQLITE_DB_PATH", filepath.Join(tmpRoot, "store.db"))
	defer func() {
		os.Unsetenv("FLOW_FILE_PATH")
		os.Unsetenv("SQLITE_DB_PATH")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("ENV")
	}()

	// Change to subdirectory (simulating GUI running from gui/)
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Failed to change to subdir: %v", err)
	}

	// Now load config - it should find .env in parent directory
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify that .env was loaded
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("Expected LOG_LEVEL=DEBUG from .env, got %s", cfg.LogLevel)
	}

	if cfg.Environment != "dev" {
		t.Errorf("Expected ENV=dev from .env, got %s", cfg.Environment)
	}
}
