package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const settingsSchemaVersion = 2

type settingsStore interface {
	Load() (map[string]any, error)
	Save(map[string]any) error
}

type fileStore struct {
	path       string
	legacyPath string
}

func newFileStore(path, legacyPath string) *fileStore {
	return &fileStore{path: path, legacyPath: legacyPath}
}

func (s *fileStore) Load() (map[string]any, error) {
	settings, err := readSettings(s.path)
	if err == nil {
		version, valid := settings["schema_version"].(float64)
		if !valid || version != settingsSchemaVersion {
			return nil, fmt.Errorf("read settings: unsupported schema_version %v", settings["schema_version"])
		}
		if migrateReservedFleetAttribute(settings) {
			if err := s.Save(settings); err != nil {
				return nil, fmt.Errorf("persist Fleet attribute migration: %w", err)
			}
			return readSettings(s.path)
		}
		return settings, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	settings = map[string]any{"schema_version": float64(settingsSchemaVersion)}
	legacy, legacyErr := readSettings(s.legacyPath)
	if legacyErr != nil && !errors.Is(legacyErr, os.ErrNotExist) {
		return nil, fmt.Errorf("read legacy settings: %w", legacyErr)
	}
	if legacyErr == nil {
		for key := range operationalSettingNames {
			if value, present := legacy[key]; present {
				settings[key] = value
			}
		}
		if _, present := settings["gcloud_rw_api_key"]; !present {
			if legacySecret, present := legacy["fleet_password"]; present {
				settings["gcloud_rw_api_key"] = legacySecret
			}
		}
	}
	if err := s.Save(settings); err != nil {
		return nil, fmt.Errorf("persist migrated settings: %w", err)
	}
	return readSettings(s.path)
}

func migrateReservedFleetAttribute(settings map[string]any) bool {
	attributes, ok := settings["fleet_attributes"].(string)
	if !ok || attributes == "" {
		return false
	}
	parts := strings.Split(attributes, ",")
	retained := make([]string, 0, len(parts))
	changed := false
	for _, part := range parts {
		key, _, hasValue := strings.Cut(part, "=")
		if hasValue && reservedFleetAttributeNames[key] {
			changed = true
			continue
		}
		retained = append(retained, part)
	}
	if !changed {
		return false
	}
	if len(retained) == 0 {
		delete(settings, "fleet_attributes")
	} else {
		settings["fleet_attributes"] = strings.Join(retained, ",")
	}
	return true
}

func (s *fileStore) Save(settings map[string]any) error {
	stored := make(map[string]any, len(settings)+1)
	for key, value := range settings {
		stored[key] = value
	}
	stored["schema_version"] = settingsSchemaVersion

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	lock, err := os.OpenFile(s.path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open settings lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock settings: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck -- closing the descriptor also releases the lock

	temporary, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary settings: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect temporary settings: %w", err)
	}
	encoder := json.NewEncoder(temporary)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(stored); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close settings: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("activate settings: %w", err)
	}
	directory, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open settings directory: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync settings directory: %w", err)
	}
	return nil
}

func readSettings(path string) (map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	var settings map[string]any
	if err := decoder.Decode(&settings); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if settings == nil {
		return nil, fmt.Errorf("decode %s: expected an object", path)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, fmt.Errorf("decode %s: multiple JSON values", path)
		}
		return nil, fmt.Errorf("decode %s: trailing data: %w", path, err)
	}
	return settings, nil
}

var operationalSettingNames = map[string]struct{}{
	"operation_mode": {}, "loki_url": {}, "loki_username": {}, "loki_password": {},
	"logs_system": {}, "logs_homeassistant": {}, "logs_addons": {},
	"logs_exclude_addons": {}, "logs_max_age": {},
	"prometheus_url": {}, "prometheus_username": {}, "prometheus_password": {},
	"instance_name": {}, "metrics_scrape_interval": {}, "host_metrics": {},
	"homeassistant_metrics": {}, "alloy_metrics": {},
	"tempo_url": {}, "tempo_username": {}, "tempo_password": {},
	"traces_enabled": {}, "traces_network_access": {},
	"pyroscope_url": {}, "pyroscope_username": {}, "pyroscope_password": {},
	"alloy_profiling": {}, "fleet_url": {}, "fleet_username": {},
	"gcloud_rw_api_key": {}, "fleet_collector_name": {}, "fleet_attributes": {}, "fleet_default_attributes": {},
	"fleet_poll_frequency": {}, "log_level": {},
	"alloy_disable_telemetry": {}, "alloy_additional_args": {},
	"additional_config": {}, "manual_config_enabled": {}, "manual_config": {},
}
