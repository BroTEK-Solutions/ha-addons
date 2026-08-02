package main

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var startupHostnamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type candidateValidator interface {
	Validate(context.Context, map[string]any) error
}

type alloyValidator struct {
	generatorPath string
	alloyPath     string
}

func newAlloyValidator(generatorPath, alloyPath string) *alloyValidator {
	return &alloyValidator{generatorPath: generatorPath, alloyPath: alloyPath}
}

func (v *alloyValidator) Validate(ctx context.Context, settings map[string]any) error {
	if err := validateModeRequirements(settings); err != nil {
		return err
	}
	if err := v.validateStartupInputs(ctx, settings); err != nil {
		return err
	}
	temporaryDir, err := os.MkdirTemp("", "alloy-candidate-*")
	if err != nil {
		return fmt.Errorf("create candidate directory: %w", err)
	}
	defer os.RemoveAll(temporaryDir)
	configPath := filepath.Join(temporaryDir, "config.alloy")

	manual, _ := settings["manual_config"].(string)
	manualEnabled, _ := settings["manual_config_enabled"].(bool)
	if manualEnabled && strings.TrimSpace(manual) != "" {
		if err := os.WriteFile(configPath, []byte(manual+"\n"), 0o600); err != nil {
			return fmt.Errorf("write manual candidate: %w", err)
		}
	} else {
		command := exec.CommandContext(ctx, "bash", v.generatorPath)
		command.Env = candidateEnvironment(settings)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		generated, err := command.Output()
		if err != nil {
			return fmt.Errorf("render candidate: %s", limitedMessage(stderr.String()))
		}
		if err := os.WriteFile(configPath, generated, 0o600); err != nil {
			return fmt.Errorf("write generated candidate: %w", err)
		}
	}

	stability := stringSetting(settings, "alloy_stability_level", "generally-available")
	command := exec.CommandContext(ctx, v.alloyPath, "validate", "--stability.level="+stability, configPath)
	command.Env = candidateEnvironment(settings)
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Alloy rejected candidate: %s", limitedMessage(string(output)))
	}
	return nil
}

func (v *alloyValidator) validateStartupInputs(ctx context.Context, settings map[string]any) error {
	additionalArguments := strings.Fields(stringSetting(settings, "alloy_additional_args", ""))
	if len(additionalArguments) > 0 {
		for _, argument := range additionalArguments {
			if argument == "-h" || argument == "--help" || !strings.HasPrefix(argument, "-") {
				return fmt.Errorf("additional Alloy arguments must use --flag or --flag=value syntax")
			}
		}
		arguments := append([]string{"run"}, additionalArguments...)
		arguments = append(arguments, "--help")
		command := exec.CommandContext(ctx, v.alloyPath, arguments...)
		command.Env = candidateEnvironment(settings)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("invalid additional Alloy arguments: %s", limitedMessage(string(output)))
		}
	}

	if optionEnabled(settings, "manual_config_enabled") {
		return nil
	}
	mode, _ := settings["operation_mode"].(string)
	if mode != "fleet" {
		for _, name := range []string{"loki_url", "prometheus_url", "tempo_url", "pyroscope_url"} {
			if err := validateEndpointSetting(name, stringSetting(settings, name, "")); err != nil {
				return err
			}
		}
		for _, name := range []string{"loki_username", "prometheus_username", "tempo_username", "pyroscope_username"} {
			if err := validateRiverString(name, stringSetting(settings, name, "")); err != nil {
				return err
			}
		}
	}
	if mode != "local" {
		if err := validateEndpointSetting("fleet_url", stringSetting(settings, "fleet_url", "")); err != nil {
			return err
		}
		for _, name := range []string{"fleet_username", "fleet_collector_name"} {
			if err := validateRiverString(name, stringSetting(settings, name, "")); err != nil {
				return err
			}
		}
		if err := validateFleetAttributes(stringSetting(settings, "fleet_attributes", "")); err != nil {
			return err
		}
	}
	return validateRiverString("instance_name", stringSetting(settings, "instance_name", "homeassistant"))
}

func validateEndpointSetting(name, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\"\\") || strings.ContainsFunc(value, unicode.IsSpace) {
		return fmt.Errorf("%s is not a valid HTTP(S) URL", name)
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("%s is not a valid HTTP(S) URL", name)
	}
	hostname := parsed.Hostname()
	if strings.Contains(hostname, ":") {
		if zone := strings.LastIndex(hostname, "%"); zone >= 0 {
			hostname = hostname[:zone]
		}
		if net.ParseIP(hostname) == nil {
			return fmt.Errorf("%s is not a valid HTTP(S) URL", name)
		}
	} else if !startupHostnamePattern.MatchString(hostname) {
		return fmt.Errorf("%s is not a valid HTTP(S) URL", name)
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return fmt.Errorf("%s is not a valid HTTP(S) URL", name)
		}
	}
	return nil
}

func validateRiverString(name, value string) error {
	if strings.ContainsAny(value, "\"\\") || strings.ContainsFunc(value, unicode.IsControl) {
		return fmt.Errorf("%s contains a quote, backslash or control character", name)
	}
	return nil
}

func validateFleetAttributes(value string) error {
	if value == "" {
		return nil
	}
	for _, pair := range strings.Split(value, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return fmt.Errorf("fleet_attributes entries must use non-empty key=value pairs")
		}
		if reservedFleetAttributeNames[parts[0]] {
			return fmt.Errorf("fleet_attributes key %s is reserved for App targeting", parts[0])
		}
		if strings.ContainsAny(pair, "\"\\") {
			return fmt.Errorf("fleet_attributes entries cannot contain quotes or backslashes")
		}
	}
	return nil
}

var reservedFleetAttributeNames = map[string]bool{
	"ha_addon_instance":           true,
	"haos":                        true,
	"journal_path":                true,
	"alloy_container_name":        true,
	"alloy_legacy_container_name": true,
	"ha_addon_slug":               true,
}

func candidateEnvironment(settings map[string]any) []string {
	environment := os.Environ()
	for setting, variable := range map[string]string{
		"operation_mode": "OPERATION_MODE", "loki_url": "LOKI_URL",
		"loki_username": "LOKI_USERNAME", "loki_password": "LOKI_PASSWORD",
		"logs_system": "LOGS_SYSTEM", "logs_homeassistant": "LOGS_HOMEASSISTANT",
		"logs_addons": "LOGS_ADDONS", "logs_exclude_addons": "LOGS_EXCLUDE_ADDONS",
		"logs_max_age": "LOGS_MAX_AGE", "prometheus_url": "PROMETHEUS_URL",
		"prometheus_username": "PROMETHEUS_USERNAME", "prometheus_password": "PROMETHEUS_PASSWORD",
		"instance_name": "INSTANCE_NAME", "metrics_scrape_interval": "METRICS_SCRAPE_INTERVAL",
		"host_metrics": "HOST_METRICS", "homeassistant_metrics": "HOMEASSISTANT_METRICS",
		"alloy_metrics": "ALLOY_METRICS", "tempo_url": "TEMPO_URL",
		"tempo_username": "TEMPO_USERNAME", "tempo_password": "TEMPO_PASSWORD",
		"traces_enabled": "TRACES_ENABLED", "traces_network_access": "TRACES_NETWORK_ACCESS",
		"pyroscope_url": "PYROSCOPE_URL", "pyroscope_username": "PYROSCOPE_USERNAME",
		"pyroscope_password": "PYROSCOPE_PASSWORD", "alloy_profiling": "ALLOY_PROFILING",
		"fleet_url": "FLEET_URL", "fleet_username": "FLEET_USERNAME",
		"gcloud_rw_api_key": "GCLOUD_RW_API_KEY", "fleet_collector_name": "FLEET_COLLECTOR_NAME",
		"fleet_attributes": "FLEET_ATTRIBUTES", "fleet_default_attributes": "FLEET_DEFAULT_ATTRIBUTES", "fleet_poll_frequency": "FLEET_POLL_FREQUENCY",
		"log_level": "LOG_LEVEL", "additional_config": "ADDITIONAL_CONFIG",
	} {
		if value, present := settings[setting]; present && value != nil {
			environment = append(environment, variable+"="+fmt.Sprint(value))
		}
	}
	return environment
}

func stringSetting(settings map[string]any, key, fallback string) string {
	if value, ok := settings[key].(string); ok && value != "" {
		return value
	}
	return fallback
}

func limitedMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 4096 {
		return message[:4096] + "…"
	}
	if message == "" {
		return "validation command failed without output"
	}
	return message
}
