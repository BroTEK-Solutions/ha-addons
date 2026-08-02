package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type fleetReferenceRenderer interface {
	Render(context.Context, map[string]any) ([]byte, error)
}

// fleetStarterSelection is deliberately separate from stored add-on settings.
// It contains the non-secret inputs needed to render one Fleet starter
// pipeline, and is never persisted.
type fleetStarterSelection struct {
	LokiURL               string `json:"loki_url"`
	LokiUsername          string `json:"loki_username"`
	PrometheusURL         string `json:"prometheus_url"`
	PrometheusUsername    string `json:"prometheus_username"`
	TempoURL              string `json:"tempo_url"`
	TempoUsername         string `json:"tempo_username"`
	PyroscopeURL          string `json:"pyroscope_url"`
	PyroscopeUsername     string `json:"pyroscope_username"`
	HostMetrics           bool   `json:"host_metrics"`
	HomeAssistantMetrics  bool   `json:"homeassistant_metrics"`
	AlloyMetrics          bool   `json:"alloy_metrics"`
	LogsSystem            bool   `json:"logs_system"`
	LogsHomeAssistant     bool   `json:"logs_homeassistant"`
	LogsAddons            bool   `json:"logs_addons"`
	LogsExcludeAddons     string `json:"logs_exclude_addons"`
	LogsMaxAge            string `json:"logs_max_age"`
	MetricsScrapeInterval string `json:"metrics_scrape_interval"`
	TracesEnabled         bool   `json:"traces_enabled"`
	TracesNetworkAccess   bool   `json:"traces_network_access"`
	AlloyProfiling        bool   `json:"alloy_profiling"`
}

func fleetStarterSettings(stored map[string]any, selection fleetStarterSelection) (map[string]any, error) {
	if optionEnabled(stored, "manual_config_enabled") {
		return nil, errors.New("disable the full manual configuration override before generating a Fleet starter pipeline")
	}
	if mode, _ := stored["operation_mode"].(string); mode != "fleet" {
		return nil, errors.New("Fleet Management must be the active operation mode before generating a Fleet starter pipeline")
	}

	settings := make(map[string]any, 32)
	for _, key := range []string{
		"operation_mode", "instance_name", "fleet_url", "fleet_username",
		"fleet_collector_name", "fleet_attributes", "fleet_poll_frequency",
		"gcloud_rw_api_key", "fleet_password",
	} {
		if value, ok := stored[key]; ok {
			settings[key] = value
		}
	}
	for key, value := range map[string]any{
		"loki_url":                selection.LokiURL,
		"loki_username":           selection.LokiUsername,
		"prometheus_url":          selection.PrometheusURL,
		"prometheus_username":     selection.PrometheusUsername,
		"tempo_url":               selection.TempoURL,
		"tempo_username":          selection.TempoUsername,
		"pyroscope_url":           selection.PyroscopeURL,
		"pyroscope_username":      selection.PyroscopeUsername,
		"host_metrics":            selection.HostMetrics,
		"homeassistant_metrics":   selection.HomeAssistantMetrics,
		"alloy_metrics":           selection.AlloyMetrics,
		"logs_system":             selection.LogsSystem,
		"logs_homeassistant":      selection.LogsHomeAssistant,
		"logs_addons":             selection.LogsAddons,
		"logs_exclude_addons":     selection.LogsExcludeAddons,
		"logs_max_age":            selection.LogsMaxAge,
		"metrics_scrape_interval": selection.MetricsScrapeInterval,
		"traces_enabled":          selection.TracesEnabled,
		"traces_network_access":   selection.TracesNetworkAccess,
		"alloy_profiling":         selection.AlloyProfiling,
	} {
		settings[key] = value
	}
	if err := validateFleetReferenceSettings(settings); err != nil {
		return nil, err
	}
	return settings, nil
}

type commandFleetReferenceRenderer struct {
	generatorPath string
	validator     candidateValidator
	journalPath   string
}

func newCommandFleetReferenceRenderer(generatorPath string, validator candidateValidator) *commandFleetReferenceRenderer {
	return &commandFleetReferenceRenderer{
		generatorPath: generatorPath,
		validator:     validator,
		journalPath:   detectJournalPath(),
	}
}

func detectJournalPath() string {
	entries, err := os.ReadDir("/var/log/journal")
	if err == nil && len(entries) > 0 {
		return "/var/log/journal"
	}
	return "/run/log/journal"
}

func (r *commandFleetReferenceRenderer) Render(ctx context.Context, settings map[string]any) ([]byte, error) {
	if err := validateFleetReferenceSettings(settings); err != nil {
		return nil, err
	}
	settings, placeholders := withFleetReferencePlaceholders(settings)

	// Validate the selected components as a Local candidate. The generated Fleet
	// pipeline uses the shared key, so supply it as each selected backend's
	// validation-only password without persisting duplicate secrets.
	candidate := make(map[string]any, len(settings)+4)
	for key, value := range settings {
		candidate[key] = value
	}
	candidate["operation_mode"] = "local"
	delete(candidate, "additional_config")
	sharedKey, _ := settings["gcloud_rw_api_key"].(string)
	if sharedKey == "" {
		// Nothing is stored yet. The rendered pipeline only ever references the
		// key by name, so a stand-in satisfies the paired-credential rule.
		sharedKey = fleetPlaceholderMarker
	}
	for _, prefix := range []string{"loki", "prometheus", "tempo", "pyroscope"} {
		delete(candidate, prefix+"_password")
		if optionHasValue(candidate, prefix+"_username") {
			candidate[prefix+"_password"] = sharedKey
		}
	}
	if err := r.validator.Validate(ctx, candidate); err != nil {
		return nil, fmt.Errorf("validate Fleet starter pipeline: %w", err)
	}

	command := exec.CommandContext(ctx, "bash", r.generatorPath)
	command.Env = append(candidateEnvironment(settings),
		"FLEET_REFERENCE_PIPELINE=true",
		"JOURNAL_PATH="+r.journalPath,
	)
	var stderr strings.Builder
	command.Stderr = &stderr
	contents, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("render Fleet starter pipeline: %s", limitedMessage(stderr.String()))
	}
	return buildFleetManifest(settings, contents, placeholders)
}

func validateFleetReferenceSettings(settings map[string]any) error {
	if stringSetting(settings, "operation_mode", "") != "fleet" {
		return errors.New("Fleet starter pipeline is available only in Fleet Management mode")
	}
	if optionEnabled(settings, "manual_config_enabled") {
		return errors.New("disable the full manual configuration override before generating a Fleet starter pipeline")
	}
	for _, destination := range fleetReferenceDestinations {
		if destination.selected(settings) {
			return nil
		}
	}
	return errors.New("select at least one metrics, logs, traces or profiles option before generating a Fleet starter pipeline")
}

// fleetPlaceholderMarker is the literal an operator has to search for and replace.
// It is deliberately loud and, in a hostname, sits under the reserved .invalid TLD
// so a manifest published unedited fails to resolve instead of shipping telemetry
// somewhere unintended.
const fleetPlaceholderMarker = "REPLACE-ME"

// A destination belongs in the starter pipeline when its signal is selected, not
// when its endpoint happens to be filled in. Anything the operator left blank is
// rendered as a placeholder for them to edit before creating the pipeline.
var fleetReferenceDestinations = []struct {
	prefix   string
	label    string
	url      string
	selected func(map[string]any) bool
}{
	{
		prefix: "prometheus",
		label:  "metrics",
		url:    "https://" + fleetPlaceholderMarker + ".invalid/api/prom/push",
		selected: func(settings map[string]any) bool {
			return optionEnabled(settings, "host_metrics") ||
				optionEnabled(settings, "homeassistant_metrics") ||
				optionEnabled(settings, "alloy_metrics")
		},
	},
	{
		prefix: "loki",
		label:  "logs",
		url:    "https://" + fleetPlaceholderMarker + ".invalid/loki/api/v1/push",
		selected: func(settings map[string]any) bool {
			return optionEnabled(settings, "logs_system") ||
				optionEnabled(settings, "logs_homeassistant") ||
				optionEnabled(settings, "logs_addons")
		},
	},
	{
		prefix:   "tempo",
		label:    "traces",
		url:      "https://" + fleetPlaceholderMarker + ".invalid/otlp",
		selected: func(settings map[string]any) bool { return optionEnabled(settings, "traces_enabled") },
	},
	{
		prefix:   "pyroscope",
		label:    "profiles",
		url:      "https://" + fleetPlaceholderMarker + ".invalid",
		selected: func(settings map[string]any) bool { return optionEnabled(settings, "alloy_profiling") },
	},
}

// withFleetReferencePlaceholders fills the endpoint and tenant of every selected
// signal that has no configured value, and reports which ones were substituted.
// Configured values are never overwritten.
func withFleetReferencePlaceholders(settings map[string]any) (map[string]any, []string) {
	filled := make(map[string]any, len(settings)+len(fleetReferenceDestinations)*2)
	for key, value := range settings {
		filled[key] = value
	}
	var placeholders []string
	for _, destination := range fleetReferenceDestinations {
		if !destination.selected(settings) {
			continue
		}
		substituted := false
		if !optionHasValue(filled, destination.prefix+"_url") {
			filled[destination.prefix+"_url"] = destination.url
			substituted = true
		}
		if !optionHasValue(filled, destination.prefix+"_username") {
			filled[destination.prefix+"_username"] = fleetPlaceholderMarker + "-" + destination.label + "-tenant-id"
			substituted = true
		}
		if substituted {
			placeholders = append(placeholders, destination.label)
		}
	}
	return filled, placeholders
}

func buildFleetManifest(settings map[string]any, contents []byte, placeholders []string) ([]byte, error) {
	contents = []byte(strings.TrimSuffix(string(contents), "\n"))
	if len(strings.TrimSpace(string(contents))) == 0 {
		return nil, errors.New("Fleet starter pipeline has no enabled telemetry components")
	}
	if len(placeholders) > 0 {
		contents = append([]byte(fmt.Sprintf(
			"// Replace every %s value below with the endpoint and numeric tenant ID for\n"+
				"// %s from your Grafana Cloud stack before you create this pipeline.\n"+
				"// The stack details are on the Grafana Cloud portal under each service.\n",
			fleetPlaceholderMarker, strings.Join(placeholders, ", "))), contents...)
	}
	instanceName := stringSetting(settings, "instance_name", "homeassistant")
	pipelineName := "home-assistant-" + fleetNameSlug(instanceName)
	if pipelineName == "home-assistant-" {
		return nil, errors.New("instance name must contain a letter or number")
	}

	var manifest strings.Builder
	fmt.Fprintf(&manifest, "apiVersion: fleet.ext.grafana.app/v1alpha1\nkind: Pipeline\nmetadata:\n  name: %s\nspec:\n  name: %s\n  enabled: true\n  contents: |-\n", pipelineName, pipelineName)
	for _, line := range strings.Split(string(contents), "\n") {
		fmt.Fprintf(&manifest, "    %s\n", line)
	}
	fmt.Fprintf(&manifest, "  matchers:\n    - %s\n", strconv.Quote("ha_addon_instance="+instanceName))
	return []byte(manifest.String()), nil
}

func fleetNameSlug(value string) string {
	var slug strings.Builder
	separator := false
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			if separator && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			slug.WriteRune(character)
			separator = false
		} else {
			separator = true
		}
	}
	base := strings.Trim(slug.String(), "-")
	digest := sha256.Sum256([]byte(value))
	suffix := fmt.Sprintf("%x", digest[:5])
	if base == "" {
		return suffix
	}
	const pipelinePrefix = "home-assistant-"
	maxBaseLength := 63 - len(pipelinePrefix) - 1 - len(suffix)
	if len(base) > maxBaseLength {
		base = strings.TrimRight(base[:maxBaseLength], "-")
	}
	return base + "-" + suffix
}
