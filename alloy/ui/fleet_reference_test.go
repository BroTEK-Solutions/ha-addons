package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rejectAdditionalConfigValidator struct{}

func (rejectAdditionalConfigValidator) Validate(_ context.Context, settings map[string]any) error {
	if optionHasValue(settings, "additional_config") {
		return errors.New("additional_config reached Fleet reference validation")
	}
	return validateModeRequirements(settings)
}

// recordingValidator keeps the real mode requirements - including the rule that a
// username and password are set together - so a placeholder render still has to
// produce a candidate Local configuration that would validate.
type recordingValidator struct{ settings map[string]any }

func (v *recordingValidator) Validate(_ context.Context, settings map[string]any) error {
	v.settings = settings
	return validateModeRequirements(settings)
}

func TestFleetManifestTargetsOnlyTheSelectedCollectorWithoutEmbeddingSecrets(t *testing.T) {
	settings := map[string]any{
		"instance_name":     "Kitchen HA",
		"gcloud_rw_api_key": "SENTINEL-SECRET",
	}
	contents := []byte("prometheus.remote_write \"metrics\" {\n  endpoint {\n    password = sys.env(\"GCLOUD_RW_API_KEY\")\n  }\n}\n")

	manifest, err := buildFleetManifest(settings, contents, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `apiVersion: fleet.ext.grafana.app/v1alpha1
kind: Pipeline
metadata:
  name: home-assistant-kitchen-ha-9381cbdd7b
spec:
  name: home-assistant-kitchen-ha-9381cbdd7b
  enabled: true
  contents: |-
    prometheus.remote_write "metrics" {
      endpoint {
        password = sys.env("GCLOUD_RW_API_KEY")
      }
    }
  matchers:
    - "ha_addon_instance=Kitchen HA"
`
	if string(manifest) != want {
		t.Fatalf("manifest =\n%s\nwant:\n%s", manifest, want)
	}
	if bytes.Contains(manifest, []byte("SENTINEL-SECRET")) {
		t.Fatal("stored secret reached Fleet manifest")
	}
}

func TestFleetNameSlugUsesManifestSafeASCII(t *testing.T) {
	if got, want := fleetNameSlug("Küche / Upstairs"), "k-che-upstairs-0b1d11e2ef"; got != want {
		t.Fatalf("fleetNameSlug() = %q, want %q", got, want)
	}
	if got, want := fleetNameSlug("家庭"), "a70a77c75b"; got != want {
		t.Fatalf("fleetNameSlug() for non-ASCII name = %q, want %q", got, want)
	}
	if fleetNameSlug("ha.one") == fleetNameSlug("ha-one") {
		t.Fatal("distinct instance names produced the same Fleet pipeline slug")
	}
	if got := "home-assistant-" + fleetNameSlug(strings.Repeat("long-name-", 20)); len(got) > 63 {
		t.Fatalf("pipeline name is %d characters: %q", len(got), got)
	}
}

func TestFleetReferencePlaceholdersFillOnlyBlankSelectedDestinations(t *testing.T) {
	settings := map[string]any{
		"operation_mode":      "fleet",
		"host_metrics":        true,
		"prometheus_url":      "https://prom.example/api/prom/push",
		"logs_system":         true,
		"traces_enabled":      true,
		"tempo_username":      "789",
		"alloy_profiling":     false,
		"pyroscope_url":       "https://profiles.example",
		"prometheus_username": "",
	}

	filled, placeholders := withFleetReferencePlaceholders(settings)

	if got := filled["prometheus_url"]; got != "https://prom.example/api/prom/push" {
		t.Errorf("configured endpoint was overwritten: %v", got)
	}
	if got := filled["tempo_username"]; got != "789" {
		t.Errorf("configured tenant was overwritten: %v", got)
	}
	for key, want := range map[string]string{
		"prometheus_username": "REPLACE-ME-metrics-tenant-id",
		"loki_url":            "https://REPLACE-ME.invalid/loki/api/v1/push",
		"loki_username":       "REPLACE-ME-logs-tenant-id",
		"tempo_url":           "https://REPLACE-ME.invalid/otlp",
	} {
		if got := filled[key]; got != want {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}
	// Profiles are not selected, so its blank tenant stays blank and no profiles
	// pipeline is rendered at all.
	if optionHasValue(filled, "pyroscope_username") {
		t.Errorf("unselected signal gained a placeholder tenant: %v", filled["pyroscope_username"])
	}
	if got, want := strings.Join(placeholders, ","), "metrics,logs,traces"; got != want {
		t.Errorf("placeholders = %q, want %q", got, want)
	}
	if optionHasValue(settings, "loki_url") {
		t.Error("withFleetReferencePlaceholders mutated the caller's settings")
	}
}

func TestFleetManifestBannerNamesThePlaceholdersToReplace(t *testing.T) {
	manifest, err := buildFleetManifest(
		map[string]any{"instance_name": "homeassistant"},
		[]byte(`loki.write "loki" {}`),
		[]string{"metrics", "logs"},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"    // Replace every REPLACE-ME value below",
		"    // metrics, logs from your Grafana Cloud stack",
		`    loki.write "loki" {}`,
	} {
		if !bytes.Contains(manifest, []byte(expected)) {
			t.Errorf("manifest missing %q:\n%s", expected, manifest)
		}
	}
}

func TestCommandFleetRendererBuildsAValidatedManifestFromFleetSelections(t *testing.T) {
	generator := filepath.Join(t.TempDir(), "generate.sh")
	script := `#!/bin/sh
set -eu
[ "$OPERATION_MODE" = fleet ]
[ "$FLEET_REFERENCE_PIPELINE" = true ]
[ "$JOURNAL_PATH" = /run/log/journal ]
printf '%s\n' 'loki.write "loki" {' '  endpoint {' '    password = sys.env("GCLOUD_RW_API_KEY")' '  }' '}'
`
	if err := os.WriteFile(generator, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	renderer := newCommandFleetReferenceRenderer(generator, rejectAdditionalConfigValidator{})
	renderer.journalPath = "/run/log/journal"
	settings := map[string]any{
		"operation_mode":        "fleet",
		"instance_name":         "homeassistant",
		"fleet_url":             "https://fleet.example",
		"fleet_username":        "123",
		"gcloud_rw_api_key":     "SENTINEL-SECRET",
		"loki_url":              "https://logs.example/loki/api/v1/push",
		"loki_username":         "123",
		"logs_homeassistant":    true,
		"additional_config":     `prometheus.remote_write "retained" {}`,
		"prometheus_password":   "retained-local-password",
		"manual_config_enabled": false,
	}

	manifest, err := renderer.Render(context.Background(), settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"kind: Pipeline",
		"name: home-assistant-homeassistant-fa232a3742",
		`password = sys.env("GCLOUD_RW_API_KEY")`,
		`- "ha_addon_instance=homeassistant"`,
	} {
		if !bytes.Contains(manifest, []byte(expected)) {
			t.Errorf("manifest missing %q:\n%s", expected, manifest)
		}
	}
	if bytes.Contains(manifest, []byte("SENTINEL-SECRET")) {
		t.Fatal("stored secret reached rendered manifest")
	}
}

func TestCommandFleetRendererRejectsAnEmptyStarterPipeline(t *testing.T) {
	renderer := newCommandFleetReferenceRenderer("unused", fakeValidator{})
	_, err := renderer.Render(context.Background(), map[string]any{
		"operation_mode":    "fleet",
		"fleet_url":         "https://fleet.example",
		"fleet_username":    "123",
		"gcloud_rw_api_key": "secret",
	})
	if err == nil || !strings.Contains(err.Error(), "select at least one") {
		t.Fatalf("Render() error = %v, want no selected signal", err)
	}
}

func TestCommandFleetRendererRendersPlaceholdersWithNoConfiguredDestination(t *testing.T) {
	generator := filepath.Join(t.TempDir(), "generate.sh")
	script := `#!/bin/sh
set -eu
printf '%s\n' "url = \"${PROMETHEUS_URL}\"" "username = \"${PROMETHEUS_USERNAME}\""
`
	if err := os.WriteFile(generator, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	renderer := newCommandFleetReferenceRenderer(generator, &recordingValidator{})
	// Nothing is stored beyond the mode and the selected signal: no endpoint, no
	// tenant, no shared key. Generation must still succeed.
	manifest, err := renderer.Render(context.Background(), map[string]any{
		"operation_mode": "fleet",
		"host_metrics":   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`url = "https://REPLACE-ME.invalid/api/prom/push"`,
		`username = "REPLACE-ME-metrics-tenant-id"`,
		"// Replace every REPLACE-ME value below",
	} {
		if !bytes.Contains(manifest, []byte(expected)) {
			t.Errorf("manifest missing %q:\n%s", expected, manifest)
		}
	}
}
