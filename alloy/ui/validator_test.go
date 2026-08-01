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

func TestAlloyValidatorRejectsStartupOnlyInputs(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{
			name: "malformed Fleet attributes",
			settings: map[string]any{
				"operation_mode": "fleet", "fleet_url": "https://fleet.example",
				"fleet_username": "123", "gcloud_rw_api_key": "secret", "fleet_attributes": "foo",
			},
			want: "key=value",
		},
		{
			name: "invalid additional flag",
			settings: map[string]any{
				"manual_config_enabled": true, "manual_config": "logging {}", "alloy_additional_args": "--bogus",
			},
			want: "unknown flag",
		},
		{
			name: "URL rejected by startup",
			settings: map[string]any{
				"operation_mode": "local", "loki_url": "https://logs.example/has space",
			},
			want: "valid HTTP(S) URL",
		},
		{
			name: "out of range endpoint port",
			settings: map[string]any{
				"operation_mode": "local", "loki_url": "https://logs.example:65536/push",
			},
			want: "valid HTTP(S) URL",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			alloy := filepath.Join(dir, "alloy")
			script := `#!/usr/bin/env bash
set -eu
if [ "${1:-}" = run ]; then
  case " $* " in *" --bogus "*) echo "unknown flag: --bogus" >&2; exit 1;; esac
  exit 0
fi
exit 0
`
			if err := os.WriteFile(alloy, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			validator := newAlloyValidator("../rootfs/usr/share/alloy/generate-config.sh", alloy)
			err := validator.Validate(context.Background(), test.settings)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() error = %v, want %q", err, test.want)
			}
		})
	}
}
