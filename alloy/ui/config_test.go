package main

import (
	"reflect"
	"testing"
)

func TestProjectConfigInfersModesWithoutExposingSecrets(t *testing.T) {
	tests := []struct {
		name       string
		options    map[string]any
		wantMode   string
		wantLegacy bool
	}{
		{"fleet", map[string]any{"fleet_url": "https://fleet.example", "gcloud_rw_api_key": "fleet-secret"}, "fleet", false},
		{"local", map[string]any{"loki_url": "https://logs.example", "loki_password": "logs-secret"}, "local", false},
		{"legacy hybrid", map[string]any{"fleet_url": "https://fleet.example", "prometheus_url": "https://metrics.example"}, "legacy-hybrid", true},
		{"explicit selection wins", map[string]any{"operation_mode": "local", "fleet_url": "https://fleet.example"}, "local", false},
		{"legacy Fleet password is shown as shared key configured", map[string]any{"fleet_url": "https://fleet.example", "fleet_password": "old-secret"}, "fleet", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := projectConfig(test.options)
			if got.Mode != test.wantMode || got.LegacyHybrid != test.wantLegacy {
				t.Fatalf("mode = %q legacy=%v, want %q legacy=%v", got.Mode, got.LegacyHybrid, test.wantMode, test.wantLegacy)
			}
			for _, key := range secretOptionNames {
				if _, exposed := got.Options[key]; exposed {
					t.Fatalf("secret %q was exposed in options", key)
				}
			}
			if _, encoded := got.Options["gcloud_rw_api_key"]; encoded {
				t.Fatal("Fleet key value reached the browser projection")
			}
			if test.name == "legacy Fleet password is shown as shared key configured" && !got.Secrets["gcloud_rw_api_key"] {
				t.Fatal("legacy Fleet password was not projected as a configured shared key")
			}
		})
	}
}

func TestMergeOptionsPreservesOmittedSecretsAndAppliesExplicitUpdates(t *testing.T) {
	current := map[string]any{
		"operation_mode":      "local",
		"loki_password":       "keep-me",
		"prometheus_password": "replace-me",
		"tempo_password":      "clear-me",
	}
	submitted := map[string]any{"operation_mode": "fleet", "fleet_url": "https://fleet.example"}
	replacement := "new-value"
	clear := ""

	got := mergeOptions(current, submitted, map[string]*string{
		"prometheus_password": &replacement,
		"tempo_password":      &clear,
	})
	want := map[string]any{
		"operation_mode":      "fleet",
		"fleet_url":           "https://fleet.example",
		"loki_password":       "keep-me",
		"prometheus_password": "new-value",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged options = %#v, want %#v", got, want)
	}
}
