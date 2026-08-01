package main

import "errors"

var secretOptionNames = []string{
	"loki_password",
	"prometheus_password",
	"tempo_password",
	"pyroscope_password",
	"gcloud_rw_api_key",
	"fleet_password",
}

type browserConfig struct {
	Options      map[string]any  `json:"options"`
	Secrets      map[string]bool `json:"secrets"`
	Mode         string          `json:"mode"`
	LegacyHybrid bool            `json:"legacy_hybrid"`
}

func projectConfig(options map[string]any) browserConfig {
	projected := make(map[string]any, len(options))
	configured := make(map[string]bool, len(secretOptionNames))
	for key, value := range options {
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
	if !explicit || (mode != "fleet" && mode != "local") {
		mode = inferMode(options)
	}
	return browserConfig{
		Options:      projected,
		Secrets:      configured,
		Mode:         mode,
		LegacyHybrid: mode == "legacy-hybrid",
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
