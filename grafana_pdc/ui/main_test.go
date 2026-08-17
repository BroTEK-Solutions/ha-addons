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

func writeOptions(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "options.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadOptionsIgnoresUnknownAndSecretFields(t *testing.T) {
	path := writeOptions(t, `{"signing_token":"super-secret","cluster":"example",
		"hosted_grafana_id":"123","some_future_option":true}`)
	opts, err := loadOptions(path)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Cluster != "example" || opts.HostedGrafanaID != "123" {
		t.Fatalf("expected the displayed fields to decode, got %+v", opts)
	}
	// An option added later must not break the page.
	encoded, err := json.Marshal(opts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "super-secret") {
		t.Fatal("the signing token must never reach the browser payload")
	}
}

func TestCheckEndpointReportsAReachableDestination(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			_ = connection.Close()
		}
	}()

	dialer := &net.Dialer{}
	status := checkEndpoint(context.Background(), dialer.DialContext, listener.Addr().String())
	if !status.Reachable {
		t.Fatalf("expected the listener to be reachable, got %+v", status)
	}
}

func TestCheckEndpointExplainsWhyADestinationFailed(t *testing.T) {
	refuse := func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dial tcp 127.0.0.1:9: connect: connection refused")
	}
	status := checkEndpoint(context.Background(), refuse, "database.internal:5432")
	if status.Reachable {
		t.Fatal("a refused connection is not reachable")
	}
	if !strings.Contains(status.Detail, "nothing is listening") {
		t.Fatalf("the detail should be actionable, got %q", status.Detail)
	}

	missing := func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("dial tcp: lookup nope.invalid: no such host")
	}
	if detail := checkEndpoint(context.Background(), missing, "nope.invalid:443").Detail; !strings.Contains(detail, "does not resolve") {
		t.Fatalf("expected a resolution message, got %q", detail)
	}
}

func TestCheckEndpointSkipsPolicyValues(t *testing.T) {
	refuse := func(context.Context, string, string) (net.Conn, error) {
		t.Fatal("a policy value must not be dialled")
		return nil, nil
	}
	for _, endpoint := range []string{"any", "none", "*.internal:443"} {
		status := checkEndpoint(context.Background(), refuse, endpoint)
		if status.Reachable || !strings.Contains(status.Detail, "nothing to test") {
			t.Errorf("%s should be reported as untestable, got %+v", endpoint, status)
		}
	}
}

func TestCollectStatusReportsAgentAndEndpoints(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("pdc_up 1\n"))
	}))
	defer agent.Close()

	path := writeOptions(t, `{"cluster":"example","allowed_endpoints":["b.example:1","a.example:2"]}`)
	refuse := func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("connect: connection refused")
	}
	fixed := func() time.Time { return time.Unix(0, 0) }

	status := collectStatus(context.Background(), path, agent.URL, agent.Client(), refuse, fixed)
	if !status.AgentResponding {
		t.Error("the agent endpoint answered, so it should be reported as responding")
	}
	if len(status.Endpoints) != 2 {
		t.Fatalf("expected both endpoints, got %+v", status.Endpoints)
	}
	// Sorted so the page does not reshuffle between refreshes.
	if status.Endpoints[0].Endpoint != "a.example:2" {
		t.Errorf("endpoints should be sorted, got %+v", status.Endpoints)
	}
}

func TestCollectStatusSurvivesAMissingOptionsFile(t *testing.T) {
	status := collectStatus(
		context.Background(),
		filepath.Join(t.TempDir(), "absent.json"),
		"http://127.0.0.1:1",
		&http.Client{Timeout: time.Second},
		(&net.Dialer{}).DialContext,
		time.Now,
	)
	if status.AgentResponding {
		t.Error("an unreachable agent must not be reported as responding")
	}
	if status.Endpoints == nil {
		t.Error("endpoints must serialize as an empty list, never null")
	}
}

func TestIngressOnlyRejectsDirectNetworkAccess(t *testing.T) {
	handler := ingressOnly("172.30.32.2", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	request.RemoteAddr = "192.168.1.50:1234"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status names internal hosts, so non-ingress traffic must be refused; got %d", recorder.Code)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	request.RemoteAddr = "172.30.32.2:1234"
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("ingress traffic must be served; got %d", recorder.Code)
	}
}

func TestHealthzIsReachableWithoutIngress(t *testing.T) {
	// The container health check runs inside the container, not through ingress.
	handler := ingressOnly("172.30.32.2", http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.RemoteAddr = "127.0.0.1:5000"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestHandlerServesTheStatusAPIAndPage(t *testing.T) {
	path := writeOptions(t, `{"cluster":"example"}`)
	handler, err := newHandler(path, "http://127.0.0.1:1/metrics")
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	var status statusResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Options.Cluster != "example" {
		t.Errorf("expected the cluster to be reported, got %+v", status.Options)
	}
	if recorder.Header().Get("Content-Security-Policy") == "" {
		t.Error("the security headers must be applied")
	}

	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Private Data Source Connect") {
		t.Errorf("the status page should be served, got %d", recorder.Code)
	}
}

func TestStatusAPIRejectsNonGet(t *testing.T) {
	handler, err := newHandler(writeOptions(t, `{}`), "http://127.0.0.1:1/metrics")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/status", nil))
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}
}
