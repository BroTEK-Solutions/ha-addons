package main

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func optionsFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOptionsNeverDecodeToken(t *testing.T) {
	opts, err := loadOptions(optionsFile(t, `{"api_token":"SENTINEL","api_server_address":"example.com:443","future":true}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(opts)
	if strings.Contains(string(encoded), "SENTINEL") {
		t.Fatal("API token reached browser-facing type")
	}
}

func TestSummarizeMetricsAllowlistsTypesAndDropsSensitiveLabels(t *testing.T) {
	metrics := `sm_agent_api_connection_status 1
sm_agent_info{id="probe-secret",name="private-name",version="0.63.0"} 1
sm_agent_updater_scrapers_total{type="http",tenantId="42"} 2
sm_agent_scraper_operations_total{type="http",tenantId="42"} 12
sm_agent_scraper_errors_total{type="http",source="private-target"} 1
sm_agent_updater_scrapers_total{type="future-secret-type"} 9
sm_agent_publisher_push_total{tenantID="42",type="metrics"} 8
process_start_time_seconds 100`
	got := summarizeMetrics(parseMetrics(strings.NewReader(metrics)), time.Unix(160, 0))
	encoded, _ := json.Marshal(got)
	for _, secret := range []string{"probe-secret", "private-name", "private-target", "tenantId", "future-secret-type"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("sensitive value %q escaped allowlist: %s", secret, encoded)
		}
	}
	if !got.Connected || got.AgentVersion != "0.63.0" || got.ProcessUptime != 60 || got.Pushes != 8 {
		t.Fatalf("unexpected diagnostics: %+v", got)
	}
	if len(got.Checks) != 1 || got.Checks[0].Type != "HTTP" || got.Checks[0].Running != 2 || got.Checks[0].Executions != 12 || got.Checks[0].Errors != 1 {
		t.Fatalf("unexpected check summary: %+v", got.Checks)
	}
}

func TestCollectStatusCombinesMetricsAndReachability(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("sm_agent_api_connection_status 1\n"))
	}))
	defer agent.Close()
	dial := func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("connection refused") }
	got := collectStatus(context.Background(), optionsFile(t, `{"api_token":"secret","api_server_address":"example.com:443"}`), agent.URL, true, agent.Client(), dial, func() time.Time { return time.Unix(0, 0) })
	if !got.AgentResponding || !got.Diagnostics.Connected || !got.BrowserChecks || got.Endpoint.Reachable {
		t.Fatalf("unexpected status: %+v", got)
	}
}

func TestIngressOnlyRejectsDirectAccess(t *testing.T) {
	handler := ingressOnly("172.30.32.2", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.168.1.2:1"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("direct access returned %d", recorder.Code)
	}
}

func TestStatusHandlerHasSecurityHeaders(t *testing.T) {
	handler := newHandler(optionsFile(t, `{}`), "http://127.0.0.1:1", false)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("page response missing security policy: %d", recorder.Code)
	}
}
