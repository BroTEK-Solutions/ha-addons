package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeSupervisor struct {
	restarted bool
}

type fakeStore struct {
	settings map[string]any
	saved    map[string]any
	saves    int
}

func (f *fakeStore) Load() (map[string]any, error) { return f.settings, nil }
func (f *fakeStore) Save(settings map[string]any) error {
	f.saved = settings
	f.settings = settings
	f.saves++
	return nil
}

type fakeValidator struct{ err error }

func (f fakeValidator) Validate(context.Context, map[string]any) error { return f.err }

func TestIngressSourceRestrictionProtectsTheControlPlane(t *testing.T) {
	called := false
	handler := ingressOnly("172.30.32.2", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	direct := httptest.NewRequest(http.MethodPost, "/api/restart", nil)
	direct.RemoteAddr = "192.168.1.20:43210"
	directResponse := httptest.NewRecorder()
	handler.ServeHTTP(directResponse, direct)
	if directResponse.Code != http.StatusForbidden || called {
		t.Fatalf("direct request = %d called=%v, want 403 and no handler call", directResponse.Code, called)
	}

	ingress := httptest.NewRequest(http.MethodPost, "/api/restart", nil)
	ingress.RemoteAddr = "172.30.32.2:43210"
	ingressResponse := httptest.NewRecorder()
	handler.ServeHTTP(ingressResponse, ingress)
	if ingressResponse.Code != http.StatusNoContent || !called {
		t.Fatalf("ingress request = %d called=%v, want 204 and handler call", ingressResponse.Code, called)
	}

	health := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	called = false
	health.RemoteAddr = "127.0.0.1:43210"
	healthResponse := httptest.NewRecorder()
	handler.ServeHTTP(healthResponse, health)
	if healthResponse.Code != http.StatusNoContent || called {
		t.Fatalf("health request = %d called=%v, want 204 without control-plane call", healthResponse.Code, called)
	}
}

func (f *fakeSupervisor) Restart(context.Context) error {
	f.restarted = true
	return nil
}

func TestConfigAPIProjectsAndSavesWithoutReturningStoredSecrets(t *testing.T) {
	store := &fakeStore{settings: map[string]any{
		"operation_mode": "local",
		"loki_url":       "https://logs.example",
		"loki_username":  "123",
		"loki_password":  "stored-secret",
	}}
	alloy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "alloy")
	}))
	defer alloy.Close()
	handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, alloy.URL)
	if err != nil {
		t.Fatal(err)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if get.Code != http.StatusOK || bytes.Contains(get.Body.Bytes(), []byte("stored-secret")) {
		t.Fatalf("GET /api/config = %d %s", get.Code, get.Body.String())
	}

	requestBody := `{"options":{"operation_mode":"local","loki_url":"https://new.example","loki_username":"123"},"secrets":{}}`
	post := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(post, request)
	if post.Code != http.StatusOK {
		t.Fatalf("POST /api/config = %d %s", post.Code, post.Body.String())
	}
	if store.saved["loki_password"] != "stored-secret" || store.saved["loki_url"] != "https://new.example" || store.saved["restart_required"] != true {
		t.Fatalf("saved options = %#v", store.saved)
	}
}

func TestStatusAPIReportsRecoveryStateWhenAlloyIsUnavailable(t *testing.T) {
	t.Setenv("SAFE_MODE", "true")
	store := &fakeStore{settings: map[string]any{"manual_config_enabled": true}}
	alloy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer alloy.Close()
	handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, alloy.URL)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/status = %d %s", response.Code, response.Body.String())
	}
	var status struct {
		AlloyReady     bool `json:"alloy_ready"`
		SafeMode       bool `json:"safe_mode"`
		ManualOverride bool `json:"manual_override"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.AlloyReady || !status.SafeMode || !status.ManualOverride {
		t.Fatalf("status = %#v", status)
	}
}

func TestStatusAPIReportsComponentHealthSeparatelyFromReadiness(t *testing.T) {
	alloy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/-/ready":
			w.WriteHeader(http.StatusOK)
		case "/-/healthy":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer alloy.Close()
	handler, err := newAppHandler(&fakeStore{settings: map[string]any{}}, fakeValidator{}, &fakeSupervisor{}, alloy.URL)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /api/status = %d %s", response.Code, response.Body.String())
	}
	var status struct {
		AlloyReady   bool `json:"alloy_ready"`
		AlloyHealthy bool `json:"alloy_healthy"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !status.AlloyReady || !status.AlloyHealthy {
		t.Fatalf("status = %#v, want ready and healthy", status)
	}
}

func TestConfigAPIRejectsCandidateBeforePersistence(t *testing.T) {
	store := &fakeStore{settings: map[string]any{"operation_mode": "local", "loki_url": "https://old.example"}}
	handler, err := newAppHandler(store, fakeValidator{err: errors.New("Alloy rejected candidate")}, &fakeSupervisor{}, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(`{"options":{"operation_mode":"local","loki_url":"https://new.example"},"secrets":{}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || store.saves != 0 || store.settings["loki_url"] != "https://old.example" {
		t.Fatalf("response=%d saves=%d settings=%#v", response.Code, store.saves, store.settings)
	}
}

func TestConfigAPISavesManualOverrideWithoutOperationMode(t *testing.T) {
	store := &fakeStore{settings: map[string]any{}}
	handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(`{"options":{"manual_config_enabled":true,"manual_config":"logging {}"},"secrets":{}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || store.saves != 1 {
		t.Fatalf("response=%d body=%s saves=%d", response.Code, response.Body.String(), store.saves)
	}
}

func TestConfigAPIRejectsLegacyHybridSaveWithoutExplicitMode(t *testing.T) {
	store := &fakeStore{settings: map[string]any{
		"fleet_url": "https://fleet.example",
		"loki_url":  "https://logs.example",
	}}
	handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"options":{"fleet_url":"https://fleet.example","loki_url":"https://logs.example"},"secrets":{}}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestConfigAPIRejectsIncompleteFleetCredentials(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing username", `{"options":{"operation_mode":"fleet","fleet_url":"https://fleet.example"},"secrets":{"gcloud_rw_api_key":"secret"}}`},
		{"missing shared key", `{"options":{"operation_mode":"fleet","fleet_url":"https://fleet.example","fleet_username":"123"},"secrets":{}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{settings: map[string]any{}}
			handler, err := newAppHandler(store, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:12345")
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(test.body))
			request.Header.Set("Content-Type", "application/json")
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRestartAPIRequestsSelfRestart(t *testing.T) {
	supervisor := &fakeSupervisor{}
	handler, err := newAppHandler(&fakeStore{settings: map[string]any{}}, fakeValidator{}, supervisor, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/restart", nil))
	if response.Code != http.StatusAccepted || !supervisor.restarted {
		t.Fatalf("restart = %d called=%v", response.Code, supervisor.restarted)
	}
}

// echoFleetRenderer returns the settings it was asked to render, so a test can
// assert which values reached the renderer.
type echoFleetRenderer struct{ settings map[string]any }

func (r *echoFleetRenderer) Render(_ context.Context, settings map[string]any) ([]byte, error) {
	r.settings = settings
	return []byte("kind: Pipeline\n"), nil
}

func postFleetReference(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/fleet-reference", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestFleetReferenceAPIReturnsAnEditableManifestWithoutExposingItPublicly(t *testing.T) {
	store := &fakeStore{settings: map[string]any{
		"operation_mode":    "fleet",
		"fleet_url":         "https://fleet.example",
		"fleet_username":    "123",
		"gcloud_rw_api_key": "stored-secret",
	}}
	handler, err := newAppHandlerWithReferences(store, fakeValidator{}, &fakeSupervisor{}, &echoFleetRenderer{}, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, postFleetReference(`{"selection":{"logs_system":true,"loki_url":"https://logs.example/loki/api/v1/push"}}`))
	if response.Code != http.StatusOK {
		t.Fatalf("POST /api/fleet-reference = %d %s", response.Code, response.Body.String())
	}
	var rendered struct {
		Manifest string `json:"manifest"`
		Filename string `json:"filename"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rendered); err != nil {
		t.Fatal(err)
	}
	if rendered.Manifest != "kind: Pipeline\n" || rendered.Filename == "" {
		t.Fatalf("rendered = %#v", rendered)
	}

	// The manifest is delivered inside the ingress-only API. Nothing serves it to
	// an unauthenticated caller any more.
	publicHandler := ingressOnly("172.30.32.2", handler)
	for _, path := range []string{"/api/fleet-reference", "/fleet-pipeline/anything"} {
		publicRequest := httptest.NewRequest(http.MethodGet, path, nil)
		publicRequest.RemoteAddr = "192.168.1.20:43210"
		publicResponse := httptest.NewRecorder()
		publicHandler.ServeHTTP(publicResponse, publicRequest)
		if publicResponse.Code != http.StatusForbidden {
			t.Fatalf("public GET %s = %d, want 403", path, publicResponse.Code)
		}
	}
}

// Generating a manifest has no side effects, so neither an unapplied save nor
// safe mode has any reason to block it. Both used to.
func TestFleetReferenceAPIRendersWithoutAppliedSettingsOrHealthySafeMode(t *testing.T) {
	t.Setenv("SAFE_MODE", "true")
	renderer := &echoFleetRenderer{}
	store := &fakeStore{settings: map[string]any{
		"operation_mode":    "fleet",
		"restart_required":  true,
		"fleet_url":         "https://fleet.example",
		"fleet_username":    "123",
		"gcloud_rw_api_key": "stored-secret",
	}}
	handler, err := newAppHandlerWithReferences(store, fakeValidator{}, &fakeSupervisor{}, renderer, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	// No destination at all: only a selected signal.
	handler.ServeHTTP(response, postFleetReference(`{"selection":{"host_metrics":true}}`))
	if response.Code != http.StatusOK {
		t.Fatalf("POST /api/fleet-reference = %d %s", response.Code, response.Body.String())
	}
	if renderer.settings["host_metrics"] != true {
		t.Errorf("submitted selection did not reach the renderer: %#v", renderer.settings)
	}
	if renderer.settings["gcloud_rw_api_key"] != "stored-secret" {
		t.Errorf("stored shared key was not merged in: %#v", renderer.settings)
	}
	if store.saves != 0 {
		t.Errorf("generating a manifest persisted settings: saves=%d", store.saves)
	}
}

func TestFleetReferenceAPIRejectsSecretBearingStarterSelections(t *testing.T) {
	store := &fakeStore{settings: map[string]any{"operation_mode": "fleet", "gcloud_rw_api_key": "stored-secret"}}
	handler, err := newAppHandlerWithReferences(store, fakeValidator{}, &fakeSupervisor{}, &echoFleetRenderer{}, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, postFleetReference(`{"selection":{"loki_url":"https://logs.example/push","loki_password":"browser-secret"}}`))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/fleet-reference = %d %s, want 400", response.Code, response.Body.String())
	}
	if store.saves != 0 {
		t.Fatalf("fleet starter request persisted settings: saves=%d", store.saves)
	}
}

func TestStaticUIContainsConditionalAccessibleConfiguration(t *testing.T) {
	handler, err := newAppHandler(&fakeStore{settings: map[string]any{}}, fakeValidator{}, &fakeSupervisor{}, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, required := range []string{
		"Operation mode",
		"Fleet Management",
		"Fleet starter pipeline",
		"Generate Fleet pipeline manifest",
		"Download manifest",
		"<code>REPLACE-ME</code> placeholder",
		"gcx config check",
		"creates this pipeline once",
		"Local configuration",
		"Grafana Cloud read/write API key",
		"HAOS system logs",
		"Home Assistant Core logs",
		"Other App logs",
		"Tempo OTLP HTTP endpoint",
		"Allow OTLP clients on the HAOS network",
		"Profile Alloy itself",
		"Advanced &amp; optional configuration options",
		"Collect operating-system services such as Supervisor, NetworkManager and the kernel.",
		"Scrape Home Assistant entities and Core internals; Home Assistant's Prometheus integration must be enabled.",
		"Numeric Grafana Cloud traces tenant ID or a basic-auth username.",
		"Changes Alloy's own App log verbosity, not the logs sent to Loki.",
		"Full manual Alloy configuration override",
		"Break-glass mode replaces every generated pipeline",
		"href=\"alloy/\"",
	} {
		if !bytes.Contains([]byte(body), []byte(required)) {
			t.Errorf("UI missing %q", required)
		}
	}
	if bytes.Contains([]byte(body), []byte("href=\"/alloy/\"")) {
		t.Fatal("UI uses an ingress-breaking absolute Alloy URL")
	}
	if !bytes.Contains([]byte(body), []byte("app.css?v=")) || !bytes.Contains([]byte(body), []byte("app.js?v=")) {
		t.Fatal("UI assets are not versioned for upgrades")
	}

	asset := httptest.NewRecorder()
	handler.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if asset.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("app.js Cache-Control = %q, want no-store", asset.Header().Get("Cache-Control"))
	}
}

func TestJSONResponseShapeIsStable(t *testing.T) {
	response := apiResponse{OK: true, Message: "saved"}
	encoded, err := json.Marshal(response)
	if err != nil || string(encoded) != `{"ok":true,"message":"saved"}` {
		t.Fatalf("encoded = %s, %v", encoded, err)
	}
}
