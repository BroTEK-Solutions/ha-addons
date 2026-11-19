package main

import (
	"errors"
	"fmt"
	"strings"
)

var secretOptionNames = []string{
	"loki_password",
	"prometheus_password",
	"tempo_password",
	"pyroscope_password",
	"gcloud_rw_api_key",
	"fleet_password",
}

type browserConfig struct {
	Options         map[string]any  `json:"options"`
	Secrets         map[string]bool `json:"secrets"`
	Mode            string          `json:"mode"`
	ModeConfigured  bool            `json:"mode_configured"`
	LegacyHybrid    bool            `json:"legacy_hybrid"`
	RestartRequired bool            `json:"restart_required"`
}

func projectConfig(options map[string]any) browserConfig {
	projected := make(map[string]any, len(options))
	configured := make(map[string]bool, len(secretOptionNames))
	for key, value := range options {
		if key == "restart_required" {
			continue
		}
		if isSecretOption(key) {
			if text, ok := value.(string); ok && text != "" {
				configured[key] = true
			}
			continue
		}
		projected[key] = value
	}
	if configured["fleet_password"] && !configured["gcloud_rw_api_key"] {
		configured["gcloud_rw_api_key"] = true
	}

	mode, explicit := options["operation_mode"].(string)
	validExplicitMode := explicit && (mode == "fleet" || mode == "local")
	if !validExplicitMode {
		mode = inferMode(options)
	}
	return browserConfig{
		Options:         projected,
		Secrets:         configured,
		Mode:            mode,
		ModeConfigured:  validExplicitMode,
		LegacyHybrid:    mode == "legacy-hybrid",
		RestartRequired: optionEnabled(options, "restart_required"),
	}
}

func inferMode(options map[string]any) string {
	hasFleet := optionHasValue(options, "fleet_url")
	hasLocal := false
	for _, key := range []string{"loki_url", "prometheus_url", "tempo_url", "pyroscope_url"} {
		hasLocal = hasLocal || optionHasValue(options, key)
	}
	switch {
	case hasFleet && hasLocal:
		return "legacy-hybrid"
	case hasFleet:
		return "fleet"
	default:
		return "local"
	}
}

func optionHasValue(options map[string]any, key string) bool {
	value, ok := options[key]
	if !ok || value == nil {
		return false
	}
	text, ok := value.(string)
	return !ok || text != ""
}

func optionEnabled(options map[string]any, key string) bool {
	value, ok := options[key].(bool)
	return ok && value
}

func validateModeRequirements(options map[string]any) error {
	if optionEnabled(options, "manual_config_enabled") {
		if manual, ok := options["manual_config"].(string); !ok || strings.TrimSpace(manual) == "" {
			return errors.New("manual configuration override requires complete config.alloy contents")
		}
		return nil
	}
	mode, _ := options["operation_mode"].(string)
	switch mode {
	case "fleet":
		if !optionHasValue(options, "fleet_url") {
			return errors.New("Fleet Management requires its service URL")
		}
		if !optionHasValue(options, "fleet_username") {
			return errors.New("Fleet Management requires its numeric username")
		}
		if !optionHasValue(options, "gcloud_rw_api_key") && !optionHasValue(options, "fleet_password") {
			return errors.New("Fleet Management requires the Grafana Cloud read/write API key")
		}
	case "local":
		hasDestination := false
		for _, key := range []string{"loki_url", "prometheus_url", "tempo_url", "pyroscope_url"} {
			if optionHasValue(options, key) {
				hasDestination = true
				break
			}
		}
		if !hasDestination {
			return errors.New("Local configuration requires at least one Loki, Prometheus, Tempo, or Pyroscope destination")
		}
		if optionEnabled(options, "traces_enabled") && !optionHasValue(options, "tempo_url") {
			return errors.New("receiving traces requires a Tempo endpoint")
		}
		if optionEnabled(options, "alloy_profiling") && !optionHasValue(options, "pyroscope_url") {
			return errors.New("Alloy profiling requires a Pyroscope endpoint")
		}
		if optionEnabled(options, "homeassistant_metrics") && !optionHasValue(options, "prometheus_url") {
			return errors.New("Home Assistant metrics require a Prometheus endpoint")
		}
		// Only where the pipeline can exist, matching init-alloy: a stale
		// selection in an App without host process visibility is ignored.
		if optionEnabled(options, "process_metrics") && currentHostProcessStatus().Active && !optionHasValue(options, "prometheus_url") {
			return errors.New("host process metrics require a Prometheus endpoint")
		}
		for _, auth := range []struct {
			prefix string
			label  string
		}{
			{"loki", "Loki"},
			{"prometheus", "Prometheus"},
			{"tempo", "Tempo"},
			{"pyroscope", "Pyroscope"},
		} {
			username := optionHasValue(options, auth.prefix+"_username")
			password := optionHasValue(options, auth.prefix+"_password")
			if username != password {
				return errors.New(auth.label + " username and password must be set together")
			}
		}
	default:
		return errors.New("choose Fleet Management or Local configuration before saving")
	}
	return nil
}

func mergeOptions(current, submitted map[string]any, updates map[string]*string) map[string]any {
	merged := make(map[string]any, len(current)+len(submitted))
	sharedKeyUpdate, sharedKeyPresent := updates["gcloud_rw_api_key"]
	sharedKeyChanged := sharedKeyPresent && sharedKeyUpdate != nil
	for key, value := range current {
		if !isSecretOption(key) {
			merged[key] = value
		}
	}
	for key, value := range submitted {
		if !isSecretOption(key) {
			merged[key] = value
		}
	}
	for _, key := range secretOptionNames {
		if key == "fleet_password" && sharedKeyChanged {
			continue
		}
		if update, present := updates[key]; present && update != nil {
			if *update != "" {
				merged[key] = *update
			}
			continue
		}
		if value, present := current[key]; present {
			merged[key] = value
		}
	}
	return merged
}

func isSecretOption(key string) bool {
	for _, secret := range secretOptionNames {
		if key == secret {
			return true
		}
	}
	return false
}

var booleanSettingNames = map[string]bool{
	"logs_system": true, "logs_homeassistant": true, "logs_addons": true,
	"host_metrics": true, "homeassistant_metrics": true, "alloy_metrics": true, "process_metrics": true,
	"traces_enabled": true, "traces_network_access": true, "alloy_profiling": true,
	"fleet_default_attributes": true, "alloy_disable_telemetry": true, "manual_config_enabled": true,
}

func validateSettingType(key string, value any) error {
	if _, allowed := operationalSettingNames[key]; !allowed && !isSecretOption(key) {
		return fmt.Errorf("unknown setting %q", key)
	}
	if booleanSettingNames[key] {
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", key)
		}
	} else if _, ok := value.(string); !ok {
		return fmt.Errorf("%s must be a string", key)
	}
	return nil
}

func validateStoredSettingsTypes(settings map[string]any) error {
	for key, value := range settings {
		switch key {
		case "schema_version":
			continue // Load checks the version; Save owns this value.
		case "restart_required":
			if _, ok := value.(bool); !ok {
				return errors.New("restart_required must be a boolean")
			}
		default:
			if err := validateSettingType(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateSubmittedSettings(input saveRequest) error {
	for key, value := range input.Options {
		if isSecretOption(key) {
			return fmt.Errorf("%s must be supplied through secrets", key)
		}
		if err := validateSettingType(key, value); err != nil {
			return err
		}
	}
	for key := range input.Secrets {
		if !isSecretOption(key) {
			return fmt.Errorf("unknown secret setting %q", key)
		}
	}
	return nil
}
