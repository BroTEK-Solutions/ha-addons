package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestFileStoreLockHelper(t *testing.T) {
	if os.Getenv("ALLOY_STORE_LOCK_HELPER") != "1" {
		return
	}
	lock, err := os.OpenFile(os.Getenv("ALLOY_STORE_LOCK_PATH"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		os.Exit(2)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Getenv("ALLOY_STORE_LOCK_READY"), []byte("ready"), 0o600); err != nil {
		os.Exit(4)
	}
	for {
		if _, err := os.Stat(os.Getenv("ALLOY_STORE_LOCK_RELEASE")); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFileStoreReturnsV2DefaultsWhenNoStateExists(t *testing.T) {
	dir := t.TempDir()
	store := newFileStore(filepath.Join(dir, "settings.json"), filepath.Join(dir, "options.json"))

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, map[string]any{"schema_version": float64(2)}) {
		t.Fatalf("Load() = %#v", got)
	}
}

func TestFileStoreImportsLegacyOptionsOnlyOnce(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	legacyPath := filepath.Join(dir, "options.json")
	legacy := map[string]any{
		"operation_mode": "fleet",
		"fleet_url":      "https://fleet.example",
		"fleet_password": "legacy-secret",
		"safe_mode":      true,
		"unknown":        "discard-me",
	}
	encoded, _ := json.Marshal(legacy)
	if err := os.WriteFile(legacyPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	store := newFileStore(settingsPath, legacyPath)

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"schema_version":    float64(2),
		"operation_mode":    "fleet",
		"fleet_url":         "https://fleet.example",
		"gcloud_rw_api_key": "legacy-secret",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imported = %#v, want %#v", got, want)
	}

	legacy["fleet_url"] = "https://changed.example"
	encoded, _ = json.Marshal(legacy)
	if err := os.WriteFile(legacyPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got["fleet_url"] != "https://fleet.example" {
		t.Fatalf("existing v2 state was replaced: %#v", got)
	}
}

func TestFileStoreMigratesReservedFleetTargetingAttribute(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	stored := `{"schema_version":2,"fleet_attributes":"env=home,ha_addon_instance=old-name,haos=false,journal_path=/old,alloy_container_name=old,ha_addon_slug=old,role=hass"}`
	if err := os.WriteFile(settingsPath, []byte(stored), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newFileStore(settingsPath, filepath.Join(dir, "options.json"))

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got["fleet_attributes"] != "env=home,role=hass" {
		t.Fatalf("fleet_attributes = %#v", got["fleet_attributes"])
	}
	persisted, err := readSettings(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if persisted["fleet_attributes"] != "env=home,role=hass" {
		t.Fatalf("persisted fleet_attributes = %#v", persisted["fleet_attributes"])
	}
}

// The stability level moved to a Native App option. A copy left behind by an
// older version is not honored any more, so it must not survive in the settings
// file where it would contradict the option Supervisor holds.
func TestFileStoreDropsStoredStabilityLevel(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	stored := `{"schema_version":2,"alloy_stability_level":"experimental","instance_name":"homeassistant"}`
	if err := os.WriteFile(settingsPath, []byte(stored), 0o600); err != nil {
		t.Fatal(err)
	}
	store := newFileStore(settingsPath, filepath.Join(dir, "options.json"))

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, present := got["alloy_stability_level"]; present {
		t.Fatalf("loaded settings still carry alloy_stability_level: %#v", got)
	}
	if got["instance_name"] != "homeassistant" {
		t.Fatalf("instance_name = %#v", got["instance_name"])
	}
	persisted, err := readSettings(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := persisted["alloy_stability_level"]; present {
		t.Fatalf("persisted settings still carry alloy_stability_level: %#v", persisted)
	}
}

func TestFileStoreSavesAtomicallyWithOwnerOnlyPermissions(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	store := newFileStore(settingsPath, filepath.Join(dir, "missing-options.json"))
	if err := store.Save(map[string]any{"operation_mode": "local"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got["schema_version"] != float64(2) || got["operation_mode"] != "local" {
		t.Fatalf("saved = %#v", got)
	}

	if err := store.Save(map[string]any{"unsupported": func() {}}); err == nil {
		t.Fatal("Save() accepted a value JSON cannot encode")
	}
	got, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got["operation_mode"] != "local" {
		t.Fatalf("failed save damaged active state: %#v", got)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".settings.json.tmp-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary files after save = %v, %v", matches, err)
	}
}

func TestFileStoreSaveWaitsForTheInitializationLock(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	readyPath := filepath.Join(dir, "ready")
	releasePath := filepath.Join(dir, "release")
	command := exec.Command(os.Args[0], "-test.run=^TestFileStoreLockHelper$")
	command.Env = append(os.Environ(),
		"ALLOY_STORE_LOCK_HELPER=1",
		"ALLOY_STORE_LOCK_PATH="+settingsPath+".lock",
		"ALLOY_STORE_LOCK_READY="+readyPath,
		"ALLOY_STORE_LOCK_RELEASE="+releasePath,
	)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill() }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(readyPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("lock helper did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	done := make(chan error, 1)
	go func() {
		done <- newFileStore(settingsPath, filepath.Join(dir, "legacy.json")).Save(map[string]any{"operation_mode": "fleet"})
	}()
	select {
	case err := <-done:
		t.Fatalf("Save returned before initialization released the lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := os.WriteFile(releasePath, []byte("release"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Save did not resume after initialization released the lock")
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestFileStoreRejectsUnsupportedOrAmbiguousState(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
	}{
		{"future schema", `{"schema_version":3}`},
		{"multiple JSON values", "{\"schema_version\":2}\n{}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "settings.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			store := newFileStore(path, filepath.Join(dir, "options.json"))
			if _, err := store.Load(); err == nil {
				t.Fatal("Load() accepted invalid state")
			}
		})
	}
}
