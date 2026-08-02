package main

import (
	"reflect"
	"strings"
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
		{"invalid explicit selection is inferred but not configured", map[string]any{"operation_mode": "hybrid", "fleet_url": "https://fleet.example"}, "fleet", false},
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
			wantConfigured := test.name == "explicit selection wins"
			if got.ModeConfigured != wantConfigured {
				t.Fatalf("mode_configured = %v, want %v", got.ModeConfigured, wantConfigured)
			}
		})
	}
}

func TestProjectConfigExposesRestartRequirementOutsideOptions(t *testing.T) {
	got := projectConfig(map[string]any{"operation_mode": "fleet", "restart_required": true})
	if !got.RestartRequired {
		t.Fatal("restart requirement was not projected")
	}
	if _, exposed := got.Options["restart_required"]; exposed {
		t.Fatal("internal restart marker reached editable options")
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

func TestMergeOptionsRemovesLegacyFleetAliasWhenSharedKeyChanges(t *testing.T) {
	for name, value := range map[string]string{
		"clear":   "",
		"replace": "new-shared-key",
	} {
		t.Run(name, func(t *testing.T) {
			current := map[string]any{
				"operation_mode": "fleet",
				"fleet_password": "legacy-key",
			}
			submitted := map[string]any{"operation_mode": "fleet"}
			update := value

			got := mergeOptions(current, submitted, map[string]*string{
				"gcloud_rw_api_key": &update,
			})

			if _, present := got["fleet_password"]; present {
				t.Fatalf("legacy Fleet alias survived %s: %#v", name, got)
			}
			if value == "" {
				if _, present := got["gcloud_rw_api_key"]; present {
					t.Fatalf("cleared shared key survived: %#v", got)
				}
			} else if got["gcloud_rw_api_key"] != value {
				t.Fatalf("replacement shared key = %#v, want %q", got["gcloud_rw_api_key"], value)
			}
		})
	}
}

func TestMergeOptionsPreservesInactiveModeConfiguration(t *testing.T) {
	current := map[string]any{
		"operation_mode":       "fleet",
		"fleet_url":            "https://fleet.example",
		"fleet_username":       "123",
		"fleet_poll_frequency": "5m",
		"gcloud_rw_api_key":    "fleet-secret",
		"loki_url":             "https://old-logs.example",
	}
	submitted := map[string]any{
		"operation_mode": "local",
		"loki_url":       "https://new-logs.example",
	}

	got := mergeOptions(current, submitted, map[string]*string{})
	for key, want := range map[string]any{
		"operation_mode":       "local",
		"loki_url":             "https://new-logs.example",
		"fleet_url":            "https://fleet.example",
		"fleet_username":       "123",
		"fleet_poll_frequency": "5m",
		"gcloud_rw_api_key":    "fleet-secret",
	} {
		if got[key] != want {
			t.Errorf("merged %s = %#v, want %#v", key, got[key], want)
		}
	}
}

func TestValidateModeRequirementsRejectsInvalidLocalCrossFieldOptions(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]any
		want    string
	}{
		{
			"traces without Tempo",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "traces_enabled": true},
			"Tempo endpoint",
		},
		{
			"profiling without Pyroscope",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "alloy_profiling": true},
			"Pyroscope endpoint",
		},
		{
			"Home Assistant metrics without Prometheus",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "homeassistant_metrics": true},
			"Prometheus endpoint",
		},
		{
			"Loki username without password",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "loki_username": "123"},
			"Loki username and password",
		},
		{
			"Prometheus username without password",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "prometheus_username": "123"},
			"Prometheus username and password",
		},
		{
			"Tempo username without password",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "tempo_username": "123"},
			"Tempo username and password",
		},
		{
			"Pyroscope username without password",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "pyroscope_username": "123"},
			"Pyroscope username and password",
		},
		{
			"password without username",
			map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "loki_password": "secret"},
			"Loki username and password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateModeRequirements(test.options)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validateModeRequirements() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestValidateModeRequirementsAllowsCompleteManualOverrideWithoutDestinations(t *testing.T) {
	err := validateModeRequirements(map[string]any{
		"manual_config_enabled": true,
		"manual_config":         "logging {}",
	})
	if err != nil {
		t.Fatalf("manual override rejected: %v", err)
	}
}
