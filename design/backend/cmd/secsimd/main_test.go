package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"secsim/design/backend/internal/store"
)

func TestEnsureConfigFileCreatesMissingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "stocker-sim.yaml")

	state, err := store.NewFromFile(configPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected config to start missing, got err=%v", err)
	}

	if err := ensureConfigFile(state, configPath); err != nil {
		t.Fatalf("ensure config file: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	contents := string(data)
	if !strings.Contains(contents, "accept transfer") {
		t.Fatalf("expected seeded rules to be written, got:\n%s", contents)
	}
}

func TestEnsureConfigFileKeepsExistingFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "stocker-sim.yaml")
	original := "hsms:\n  mode: active\n"
	if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	state, err := store.NewFromFile(configPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}

	if err := ensureConfigFile(state, configPath); err != nil {
		t.Fatalf("ensure config file: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	if string(data) != original {
		t.Fatalf("expected existing config file to remain unchanged, got:\n%s", string(data))
	}
}
