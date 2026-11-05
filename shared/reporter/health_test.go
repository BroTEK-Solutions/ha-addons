package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Exercise the shipped specifications through real HTTP probes. A problem
// entity must be OFF on success, unlike a readiness/running entity.
func TestAlloyProblemSensors(t *testing.T) {
	for _, tc := range []struct {
		name, metric, wantProblem string
		status                    int
	}{
		{"healthy", "alloy_config_last_load_successful 1\n", "OFF", 200},
		{"failed", "alloy_config_last_load_successful 0\n", "ON", 503},
		{"unknown metric", "", "", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/metrics" {
					_, _ = w.Write([]byte(tc.metric))
					return
				}
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			entities, err := specsFor("alloy")
			if err != nil {
				t.Fatal(err)
			}
			for i := range entities {
				if entities[i].Probe.Kind == probeHTTPStatus {
					entities[i].Probe.URL = server.URL + "/health"
				} else {
					entities[i].Probe.URL = server.URL + "/metrics"
				}
			}
			values := newCollector(server.Client()).Collect(context.Background(), entities)
			wantHealth := "OFF"
			wantReady := "ON"
			if tc.status != 200 {
				wantHealth, wantReady = "ON", "OFF"
			}
			if values["healthy"] != wantHealth || values["ready"] != wantReady {
				t.Errorf("health/readiness polarity: got %v", values)
			}
			if tc.wantProblem == "" {
				if values["config_loaded"] != nil {
					t.Errorf("missing metric must remain unknown: %v", values)
				}
			} else if values["config_loaded"] != tc.wantProblem {
				t.Errorf("configuration problem = %v, want %s", values["config_loaded"], tc.wantProblem)
			}
		})
	}
}

func TestDiscoveryReadableModelPreservesIdentity(t *testing.T) {
	info := appInfo{Slug: "repository_alloy", Name: "Grafana Alloy", Version: "1.0.0"}
	encoded, err := buildDiscovery(entitySpec{Key: "ready", Component: componentBinarySensor}, info)
	if err != nil {
		t.Fatal(err)
	}
	var payload discoveryPayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Device.Model != "Grafana Alloy" {
		t.Errorf("model = %q, want readable product name", payload.Device.Model)
	}
	if payload.UniqueID != "brotek_repository_alloy_ready" || len(payload.Device.Identifiers) != 1 || payload.Device.Identifiers[0] != "brotek_repository_alloy" {
		t.Errorf("existing registry identity changed: %+v", payload)
	}
}
