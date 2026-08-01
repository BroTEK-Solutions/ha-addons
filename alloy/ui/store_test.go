package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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
