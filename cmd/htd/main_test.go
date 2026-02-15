package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestValidate_ExamplesConfig(t *testing.T) {
	example := filepath.Join("..", "..", "examples", "todoist.yaml")

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"validate", "-f", example})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if got := out.String(); got == "" {
		t.Fatalf("expected some output, got empty")
	}
}

func TestValidate_JSONOutput(t *testing.T) {
	example := filepath.Join("..", "..", "examples", "todoist.yaml")

	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json", "validate", "-f", example})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if got := out.String(); got != "{\"valid\":true}\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestValidate_MissingFile(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"validate", "-f", "does-not-exist.yaml"})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error, got nil")
	}
}

