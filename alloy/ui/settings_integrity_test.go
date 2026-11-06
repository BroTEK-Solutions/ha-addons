package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func settingsRequest(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/config", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestSettingsHTTPRejectsOversizedMergeWithoutDamagingStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store := newFileStore(path, path+".legacy")
	if err := store.Save(map[string]any{"additional_config": "// " + strings.Repeat("a", 800000)}); err != nil {
		t.Fatal(err)
	}
	handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	response := settingsRequest(t, handler, `{"options":{"manual_config_enabled":true,"manual_config":"// `+strings.Repeat("b", 800000)+`"}}`)
	if response.Code != http.StatusOK {
		t.Fatalf("large valid merge: %d %s", response.Code, response.Body.String())
	}
	if _, err := store.Load(context.Background()); err != nil {
		t.Fatalf("successful save became unreadable: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	response = settingsRequest(t, handler, `{"options":{"manual_config_enabled":true,"fleet_collector_name":"`+strings.Repeat("c", 600000)+`"}}`)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized merge: %d %s", response.Code, response.Body.String())
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("oversized merge replaced the previous settings")
	}
}

func TestSettingsHTTPRejectsWrongTypesUnknownKeysAndTrailingJSON(t *testing.T) {
	for _, body := range []string{
		`{"options":{"manual_config_enabled":true,"manual_config":123}}`,
		`{"options":{"manual_config_enabled":true,"manual_config":"logging {}","host_metrics":"false"}}`,
		`{"options":{"manual_config_enabled":true,"manual_config":"logging {}","unexpected":true}}`,
		`{"options":{"manual_config_enabled":true,"manual_config":"logging {}"},"secrets":{"unexpected":"secret"}}`,
		`{"options":{"manual_config_enabled":true,"manual_config":"logging {}"}} {}`,
	} {
		store := &fakeStore{settings: map[string]any{}}
		handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:1")
		if err != nil {
			t.Fatal(err)
		}
		response := settingsRequest(t, handler, body)
		if response.Code != http.StatusBadRequest || store.saves != 0 {
			t.Errorf("invalid input accepted: %s -> %d, saves=%d", body, response.Code, store.saves)
		}
	}
}

type pausedSettingsValidator struct{ entered, release chan struct{} }

func (v pausedSettingsValidator) Validate(ctx context.Context, settings map[string]any) error {
	if settings["instance_name"] == "first" {
		close(v.entered)
		select {
		case <-v.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestConcurrentHTTPSavePreservesNewSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store := newFileStore(path, path+".legacy")
	if err := store.Save(map[string]any{"operation_mode": "local", "loki_url": "https://logs.example", "loki_username": "account", "loki_password": "old"}); err != nil {
		t.Fatal(err)
	}
	validator := pausedSettingsValidator{make(chan struct{}), make(chan struct{})}
	handler, err := newAppHandler(store, validator, &fakeSupervisor{}, "http://127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	client := &http.Client{Timeout: 3 * time.Second}
	post := func(body string) error {
		response, err := client.Post(server.URL+"/api/config", "application/json", strings.NewReader(body))
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(response.Body)
			return fmt.Errorf("save status %d: %s", response.StatusCode, data)
		}
		return nil
	}
	first := make(chan error, 1)
	go func() { first <- post(`{"options":{"operation_mode":"local","instance_name":"first"}}`) }()
	select {
	case <-validator.entered:
	case <-time.After(time.Second):
		close(validator.release)
		t.Fatal("first validator never started")
	}
	second := make(chan error, 1)
	go func() {
		second <- post(`{"options":{"operation_mode":"local","instance_name":"second"},"secrets":{"loki_password":"new"}}`)
	}()
	// Before the fix the second save can finish while the first holds a stale
	// snapshot. With transaction locking it waits, then merges the latest file.
	var secondErr error
	secondDone := false
	select {
	case secondErr = <-second:
		secondDone = true
	case <-time.After(100 * time.Millisecond):
	}
	close(validator.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if !secondDone {
		secondErr = <-second
	}
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	saved, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if saved["loki_password"] != "new" {
		t.Fatal("a concurrent save restored the old secret")
	}
}

func TestSettingsSizeLimitCoversTheCompleteEncodedDocument(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	store := newFileStore(path, path+".legacy")
	overhead := len("{\"manual_config\":\"\",\"schema_version\":2}\n")
	value := strings.Repeat("x", (2<<20)-overhead)
	if err := store.Save(map[string]any{"manual_config": value}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 2<<20 {
		t.Fatalf("boundary document = %d bytes", len(before))
	}
	if err := store.Save(map[string]any{"manual_config": value + "x"}); err == nil {
		t.Fatal("accepted encoded document one byte above the limit")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("oversized save changed the boundary document")
	}
	// A valid prefix must not hide trailing bytes beyond a LimitReader's EOF.
	if err := os.WriteFile(path, []byte(`{"schema_version":2}`+strings.Repeat(" ", 2<<20)+"{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(context.Background()); err == nil {
		t.Fatal("loader ignored trailing data beyond the size limit")
	}
}
