package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadOptionsRejectsUnknownFields(t *testing.T) {
	_, err := loadOptions(strings.NewReader(`{
		"api_token":"secret",
		"api_server_address":"synthetic-monitoring-grpc-gb-south-1.grafana.net:443",
		"surprise":true
	}`))
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("loadOptions() error = %v, want unknown field error", err)
	}
}

func TestBuildAgentProcessKeepsTokenOutOfArguments(t *testing.T) {
	opts := options{
		APIToken:             "SENTINEL_TOKEN",
		APIServerAddress:     "synthetic-monitoring-grpc-gb-south-1.grafana.net:443",
		LogLevel:             "info",
		AllowPrivateNetworks: true,
		DisableUsageReports:  true,
	}

	got, err := buildAgentProcess(opts, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatalf("buildAgentProcess() error = %v", err)
	}
	wantArgs := []string{
		"/usr/local/bin/synthetic-monitoring-agent",
		"--api-server-address=synthetic-monitoring-grpc-gb-south-1.grafana.net:443",
		"--listen-address=127.0.0.1:4050",
		"--verbose=true",
		"--blocked-nets=",
		"--disable-usage-reports=true",
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", got.Args, wantArgs)
	}
	if strings.Contains(strings.Join(got.Args, "\x00"), opts.APIToken) {
		t.Fatal("probe token leaked into process arguments")
	}
	if !containsString(got.Env, "SM_AGENT_API_TOKEN=SENTINEL_TOKEN") {
		t.Fatalf("Env = %#v, want SM_AGENT_API_TOKEN", got.Env)
	}
}

func TestBuildAgentProcessUsesWarnDefaults(t *testing.T) {
	got, err := buildAgentProcess(options{
		APIToken:         "secret",
		APIServerAddress: "synthetic-monitoring-grpc.grafana.net:443",
		LogLevel:         "warn",
	}, nil)
	if err != nil {
		t.Fatalf("buildAgentProcess() error = %v", err)
	}
	want := []string{
		"/usr/local/bin/synthetic-monitoring-agent",
		"--api-server-address=synthetic-monitoring-grpc.grafana.net:443",
		"--listen-address=127.0.0.1:4050",
	}
	if !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("Args = %#v, want %#v", got.Args, want)
	}
}

func TestWrapWithTiniKeepsAgentBehindInit(t *testing.T) {
	process := agentProcess{
		Args: []string{agentPath, "--listen-address=127.0.0.1:4050"},
		Env:  []string{"PATH=/usr/bin"},
	}

	got := wrapWithTini(process, "/sbin/tini")
	wantArgs := []string{
		"/sbin/tini",
		"--",
		agentPath,
		"--listen-address=127.0.0.1:4050",
	}
	if got.Path != "/sbin/tini" {
		t.Fatalf("Path = %q, want /sbin/tini", got.Path)
	}
	if !reflect.DeepEqual(got.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", got.Args, wantArgs)
	}
	if !reflect.DeepEqual(got.Env, process.Env) {
		t.Fatalf("Env = %#v, want %#v", got.Env, process.Env)
	}
}

func TestBuildAgentProcessRejectsUnsafeConnectionSettings(t *testing.T) {
	tests := []struct {
		name    string
		options options
		want    string
	}{
		{
			name: "missing token",
			options: options{
				APIServerAddress: "synthetic-monitoring-grpc.grafana.net:443",
				LogLevel:         "warn",
			},
			want: "api_token is required",
		},
		{
			name: "URL instead of host and port",
			options: options{
				APIToken:         "secret",
				APIServerAddress: "https://synthetic-monitoring-grpc.grafana.net:443",
				LogLevel:         "warn",
			},
			want: "host:port",
		},
		{
			name: "missing port",
			options: options{
				APIToken:         "secret",
				APIServerAddress: "synthetic-monitoring-grpc.grafana.net",
				LogLevel:         "warn",
			},
			want: "host:port",
		},
		{
			name: "unsupported log level",
			options: options{
				APIToken:         "secret",
				APIServerAddress: "synthetic-monitoring-grpc.grafana.net:443",
				LogLevel:         "trace",
			},
			want: "log_level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildAgentProcess(tt.options, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("buildAgentProcess() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestHealthcheckUsesLivenessEndpoint(t *testing.T) {
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := checkHealth(server.Client(), server.URL); err != nil {
		t.Fatalf("checkHealth() error = %v", err)
	}
	if path != "/" {
		t.Fatalf("health path = %q, want liveness path / (not readiness /ready)", path)
	}
}

func TestWriteFatalDoesNotLeakToken(t *testing.T) {
	var output bytes.Buffer
	writeFatal(&output, "SENTINEL_TOKEN", "failed for SENTINEL_TOKEN")
	if strings.Contains(output.String(), "SENTINEL_TOKEN") {
		t.Fatalf("fatal output leaked token: %q", output.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestStartReporterIsANoOpWithoutAnAppName(t *testing.T) {
	t.Setenv("REPORTER_APP", "")
	var log strings.Builder
	// A missing REPORTER_APP means this image does not publish entities. It must
	// not warn, and above all must not fail: the probe still has to start.
	startReporter("/nonexistent/ha-reporter", os.Environ(), &log)
	if log.String() != "" {
		t.Fatalf("expected silence, got %q", log.String())
	}
}

func TestStartReporterIsANoOpWhenTheBinaryIsAbsent(t *testing.T) {
	t.Setenv("REPORTER_APP", "grafana_sm")
	var log strings.Builder
	startReporter(filepath.Join(t.TempDir(), "absent"), os.Environ(), &log)
	if log.String() != "" {
		t.Fatalf("an absent reporter must not be reported as a failure, got %q", log.String())
	}
}

func TestReporterNeverReceivesTheProbeToken(t *testing.T) {
	// The agent's token is added to the agent's own environment only. Whatever
	// the launcher hands the reporter must not carry it.
	opts := options{
		APIToken:         "super-secret-token",
		APIServerAddress: "example.grafana.net:443",
		LogLevel:         "warn",
	}
	process, err := buildAgentProcess(opts, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatal(err)
	}
	agentCarriesToken := false
	for _, entry := range process.Env {
		if strings.Contains(entry, opts.APIToken) {
			agentCarriesToken = true
		}
	}
	if !agentCarriesToken {
		t.Fatal("the agent must receive its token")
	}
	for _, entry := range os.Environ() {
		if strings.Contains(entry, opts.APIToken) {
			t.Fatal("the ambient environment handed to the reporter must not contain the token")
		}
	}
}

func TestStartUIIsANoOpWhenTheBinaryIsAbsent(t *testing.T) {
	var log strings.Builder
	startUI(filepath.Join(t.TempDir(), "absent"), os.Environ(), options{}, &log)
	if log.String() != "" {
		t.Fatalf("an absent UI must not prevent probe startup, got %q", log.String())
	}
}
