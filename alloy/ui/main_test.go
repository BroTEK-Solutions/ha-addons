package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSupervisor struct {
	options   map[string]any
	saved     map[string]any
	restarted bool
}

func (f *fakeSupervisor) Options(context.Context) (map[string]any, error) {
	return f.options, nil
}

func (f *fakeSupervisor) Save(_ context.Context, options map[string]any) error {
	f.saved = options
	return nil
}

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
	supervisor := &fakeSupervisor{options: map[string]any{
		"operation_mode": "local",
		"loki_url":       "https://logs.example",
		"loki_password":  "stored-secret",
	}}
	alloy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "alloy")
	}))
	defer alloy.Close()
	handler, err := newAppHandler(supervisor, alloy.URL)
	if err != nil {
		t.Fatal(err)
	}

	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if get.Code != http.StatusOK || bytes.Contains(get.Body.Bytes(), []byte("stored-secret")) {
		t.Fatalf("GET /api/config = %d %s", get.Code, get.Body.String())
	}

	requestBody := `{"options":{"operation_mode":"local","loki_url":"https://new.example"},"secrets":{}}`
	post := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/config", bytes.NewBufferString(requestBody))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(post, request)
	if post.Code != http.StatusOK {
		t.Fatalf("POST /api/config = %d %s", post.Code, post.Body.String())
	}
	if supervisor.saved["loki_password"] != "stored-secret" || supervisor.saved["loki_url"] != "https://new.example" {
		t.Fatalf("saved options = %#v", supervisor.saved)
	}
}

func TestConfigAPIRejectsLegacyHybridSaveWithoutExplicitMode(t *testing.T) {
	supervisor := &fakeSupervisor{options: map[string]any{
		"fleet_url": "https://fleet.example",
		"loki_url":  "https://logs.example",
	}}
	handler, err := newAppHandler(supervisor, "http://127.0.0.1:12345")
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
			supervisor := &fakeSupervisor{options: map[string]any{}}
			handler, err := newAppHandler(supervisor, "http://127.0.0.1:12345")
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
	supervisor := &fakeSupervisor{options: map[string]any{}}
	handler, err := newAppHandler(supervisor, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/restart", nil))
	if response.Code != http.StatusAccepted || !supervisor.restarted {
		t.Fatalf("restart = %d called=%v", response.Code, supervisor.restarted)
	}
}

func TestStaticUIContainsConditionalAccessibleConfiguration(t *testing.T) {
	supervisor := &fakeSupervisor{options: map[string]any{}}
	handler, err := newAppHandler(supervisor, "http://127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	body := response.Body.String()
	for _, required := range []string{
		"Operation mode",
		"Fleet Management",
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
		"href=\"alloy/\"",
	} {
		if !bytes.Contains([]byte(body), []byte(required)) {
			t.Errorf("UI missing %q", required)
		}
	}
	if bytes.Contains([]byte(body), []byte("href=\"/alloy/\"")) {
		t.Fatal("UI uses an ingress-breaking absolute Alloy URL")
	}
}

func TestJSONResponseShapeIsStable(t *testing.T) {
	response := apiResponse{OK: true, Message: "saved"}
	encoded, err := json.Marshal(response)
	if err != nil || string(encoded) != `{"ok":true,"message":"saved"}` {
		t.Fatalf("encoded = %s, %v", encoded, err)
	}
}
