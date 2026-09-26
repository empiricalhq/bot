package config_test

import (
	"path/filepath"
	"testing"

	"whatsbot/internal/config"
)

func TestLoadAcceptsMissingFlowFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "conversation.json")
	t.Setenv("FLOW_FILE_PATH", missing)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load with a missing flow file: %v", err)
	}

	if cfg.FlowFilePath != missing {
		t.Errorf("FlowFilePath = %q, want %q", cfg.FlowFilePath, missing)
	}
}

func TestLoadFlowFileURL(t *testing.T) {
	t.Setenv("FLOW_FILE_URL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if want := "https://raw.githubusercontent.com/empiricalhq/bot/master/conversation.json"; cfg.FlowFileURL != want {
		t.Errorf("default FlowFileURL = %q, want %q", cfg.FlowFileURL, want)
	}

	t.Setenv("FLOW_FILE_URL", "http://localhost/flow.json")

	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.FlowFileURL != "http://localhost/flow.json" {
		t.Errorf("FlowFileURL = %q, want the override", cfg.FlowFileURL)
	}
}
