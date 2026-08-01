package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type staticFleetRenderer struct {
	contents []byte
	err      error
}

func (r staticFleetRenderer) Render(context.Context, map[string]any) ([]byte, error) {
	return r.contents, r.err
}

type rejectAdditionalConfigValidator struct{}

func (rejectAdditionalConfigValidator) Validate(_ context.Context, settings map[string]any) error {
	if optionHasValue(settings, "additional_config") {
		return errors.New("additional_config reached Fleet reference validation")
	}
	return validateModeRequirements(settings)
}

func TestFleetManifestTargetsOnlyTheSelectedCollectorWithoutEmbeddingSecrets(t *testing.T) {
	settings := map[string]any{
		"instance_name":     "Kitchen HA",
		"gcloud_rw_api_key": "SENTINEL-SECRET",
	}
	contents := []byte("prometheus.remote_write \"metrics\" {\n  endpoint {\n    password = sys.env(\"GCLOUD_RW_API_KEY\")\n  }\n}\n")

	manifest, err := buildFleetManifest(settings, contents)
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

func TestFleetReferenceBrokerExpiresIssuedManifests(t *testing.T) {
	now := time.Date(2026, 8, 1, 18, 0, 0, 0, time.UTC)
	broker := newFleetReferenceBroker(
		staticFleetRenderer{contents: []byte("manifest")},
		strings.NewReader("01234567890123456789012345678901"),
		func() time.Time { return now },
		10*time.Minute,
	)

	issued, err := broker.Issue(context.Background(), map[string]any{"operation_mode": "fleet"})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token == "" || !issued.ExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("issued = %#v", issued)
	}
	if manifest, ok := broker.Get(issued.Token); !ok || string(manifest) != "manifest" {
		t.Fatalf("Get before expiry = %q, %v", manifest, ok)
	}

	now = now.Add(10*time.Minute + time.Second)
	if manifest, ok := broker.Get(issued.Token); ok || manifest != nil {
		t.Fatalf("Get after expiry = %q, %v", manifest, ok)
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
	if err == nil || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("Render() error = %v, want missing destination", err)
	}
}
