package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAlloyValidatorRendersWithoutWritingSecretsIntoConfig(t *testing.T) {
	dir := t.TempDir()
	alloy := filepath.Join(dir, "alloy")
	script := `#!/usr/bin/env bash
set -eu
config="${@: -1}"
grep -q 'sys.env("LOKI_PASSWORD")' "$config"
! grep -q 'SENTINEL-SECRET' "$config"
`
	if err := os.WriteFile(alloy, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	validator := newAlloyValidator("../rootfs/usr/share/alloy/generate-config.sh", alloy)
	settings := map[string]any{
		"operation_mode": "local",
		"loki_url":       "https://logs.example/loki/api/v1/push",
		"loki_username":  "123",
		"loki_password":  "SENTINEL-SECRET",
	}
	if err := validator.Validate(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
}

func TestAlloyValidatorRejectsInvalidManualConfiguration(t *testing.T) {
	dir := t.TempDir()
	alloy := filepath.Join(dir, "alloy")
	script := `#!/usr/bin/env bash
set -eu
config="${@: -1}"
if grep -q INVALID "$config"; then
  echo "manual config is invalid" >&2
  exit 1
fi
`
	if err := os.WriteFile(alloy, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	validator := newAlloyValidator("../rootfs/usr/share/alloy/generate-config.sh", alloy)
	err := validator.Validate(context.Background(), map[string]any{
		"operation_mode":        "local",
		"loki_url":              "https://logs.example/loki/api/v1/push",
		"manual_config_enabled": true,
		"manual_config":         "INVALID",
	})
	if err == nil || !strings.Contains(err.Error(), "manual config is invalid") {
		t.Fatalf("Validate() error = %v", err)
	}
}
