package config_test

import (
	"path/filepath"
	"testing"

	"whatsbot/internal/config"
)

func TestLoadAcceptsMissingFlowFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.json")

	t.Setenv("FLOW_FILE_PATH", missing)
	t.Setenv("SQLITE_DB_PATH", filepath.Join(t.TempDir(), "store.db"))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load with a missing flow file: %v", err)
	}

	if cfg.FlowFilePath != missing {
		t.Errorf("FlowFilePath = %q, want %q", cfg.FlowFilePath, missing)
	}
}
