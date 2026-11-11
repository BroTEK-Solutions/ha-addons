package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusAPIReportsHostProcessVisibility(t *testing.T) {
	for _, tc := range []struct {
		name    string
		variant string
		hostPID string
		want    hostProcessStatus
	}{
		{"standard App", "", "", hostProcessStatus{}},
		{"variant under protection mode", "true", "", hostProcessStatus{Variant: true}},
		{"variant with protection mode off", "true", "true", hostProcessStatus{Variant: true, Active: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ALLOY_HOST_PROCESSES_VARIANT", tc.variant)
			t.Setenv("ALLOY_HOST_PID", tc.hostPID)
			handler, err := newAppHandler(&fakeStore{settings: map[string]any{}}, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:1")
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
			var status statusResponse
			if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
				t.Fatal(err)
			}
			if status.HostProcesses != tc.want {
				t.Fatalf("host_processes = %#v, want %#v", status.HostProcesses, tc.want)
			}
		})
	}
}

func TestProcessMetricsIsABooleanSetting(t *testing.T) {
	if err := validateSettingType("process_metrics", true); err != nil {
		t.Fatalf("process_metrics=true rejected: %v", err)
	}
	if err := validateSettingType("process_metrics", "true"); err == nil {
		t.Fatal("process_metrics accepted a string")
	}
}

// Like Home Assistant metrics, a selected pipeline with nowhere to send it is
// rejected, but only where the pipeline can exist.
func TestProcessMetricsNeedAPrometheusEndpointWhereVisible(t *testing.T) {
	settings := map[string]any{"operation_mode": "local", "loki_url": "https://logs.example/push", "process_metrics": true}
	t.Setenv("ALLOY_HOST_PID", "")
	if err := validateModeRequirements(settings); err != nil {
		t.Fatalf("stale selection without visibility rejected: %v", err)
	}
	t.Setenv("ALLOY_HOST_PID", "true")
	if err := validateModeRequirements(settings); err == nil || !strings.Contains(err.Error(), "Prometheus endpoint") {
		t.Fatalf("visible selection without Prometheus: err = %v", err)
	}
}

// The candidate is rendered with the same gate init-alloy applies: the
// selection renders the exporter only in the host PID namespace.
func TestAlloyValidatorRendersProcessMetricsOnlyWhenVisibilityIsActive(t *testing.T) {
	dir := t.TempDir()
	alloy := filepath.Join(dir, "alloy")
	marker := filepath.Join(dir, "rendered")
	script := `#!/usr/bin/env bash
set -eu
config="${@: -1}"
if grep -q 'prometheus.exporter.process "processes"' "$config"; then
  echo yes >"` + marker + `"
else
  echo no >"` + marker + `"
fi
`
	if err := os.WriteFile(alloy, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	validator := newAlloyValidator("../rootfs/usr/share/alloy/generate-config.sh", alloy, "")
	settings := map[string]any{
		"operation_mode":  "local",
		"prometheus_url":  "https://prom.example/api/prom/push",
		"process_metrics": true,
	}
	for _, tc := range []struct {
		variant, hostPID, want string
	}{
		{"", "", "no"},
		{"true", "", "no"},
		{"true", "true", "yes"},
	} {
		t.Setenv("ALLOY_HOST_PROCESSES_VARIANT", tc.variant)
		t.Setenv("ALLOY_HOST_PID", tc.hostPID)
		if err := validator.Validate(context.Background(), settings); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(marker)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want+"\n" {
			t.Fatalf("variant=%q ALLOY_HOST_PID=%q rendered exporter = %q, want %q", tc.variant, tc.hostPID, got, tc.want)
		}
	}
}
