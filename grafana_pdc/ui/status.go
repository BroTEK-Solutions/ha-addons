package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// options mirrors only the fields this page shows. The signing token is
// deliberately absent: this type is serialized straight to the browser, so a
// secret that is never decoded here can never be leaked by a later change.
type options struct {
	HostedGrafanaID  string   `json:"hosted_grafana_id"`
	Cluster          string   `json:"cluster"`
	Domain           string   `json:"domain"`
	APIFQDN          string   `json:"api_fqdn"`
	GatewayFQDN      string   `json:"gateway_fqdn"`
	LogLevel         string   `json:"log_level"`
	Connections      int      `json:"connections"`
	AllowedEndpoints []string `json:"allowed_endpoints"`
}

type endpointStatus struct {
	Endpoint  string `json:"endpoint"`
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail"`
}

type statusResponse struct {
	AgentResponding bool             `json:"agent_responding"`
	Options         options          `json:"options"`
	Endpoints       []endpointStatus `json:"endpoints"`
	CheckedAt       string           `json:"checked_at"`
}

// loadOptions reads the Supervisor-managed options file. Unknown fields are
// ignored rather than rejected, because this page must keep working when the
// App gains an option it does not display.
func loadOptions(path string) (options, error) {
	file, err := os.Open(path)
	if err != nil {
		return options{}, err
	}
	defer file.Close()
	var result options
	if err := json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&result); err != nil {
		return options{}, err
	}
	return result, nil
}

// agentResponding probes the PDC agent's own metrics endpoint. It proves the
// process is alive; it does not prove the tunnel is registered, which is a fact
// only Grafana Cloud holds. The page says so rather than implying otherwise.
func agentResponding(ctx context.Context, client *http.Client, metricsURL string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return false
	}
	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode >= 200 && response.StatusCode < 300
}

// checkEndpoint answers the question this page exists for: can this App
// actually open a TCP connection to the destination Grafana will be told to
// use? A misconfigured allowlist and an unreachable data source look identical
// from Grafana Cloud, and until now the only way to tell them apart was to
// guess.
func checkEndpoint(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error), endpoint string) endpointStatus {
	// The OpenSSH sentinels and wildcards are policy, not destinations, so
	// there is nothing to dial.
	switch {
	case endpoint == "any" || endpoint == "none":
		return endpointStatus{Endpoint: endpoint, Detail: "policy value, nothing to test"}
	case strings.Contains(endpoint, "*"):
		return endpointStatus{Endpoint: endpoint, Detail: "wildcard pattern, nothing to test"}
	}

	attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	connection, err := dial(attempt, "tcp", endpoint)
	if err != nil {
		return endpointStatus{Endpoint: endpoint, Detail: summarizeDialError(err)}
	}
	_ = connection.Close()
	return endpointStatus{Endpoint: endpoint, Reachable: true, Detail: "TCP connection succeeded"}
}

// summarizeDialError turns a dial failure into something a user can act on
// without needing to read Go error formatting.
func summarizeDialError(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "no such host"):
		return "hostname does not resolve"
	case strings.Contains(message, "connection refused"):
		return "host reachable, but nothing is listening on that port"
	case strings.Contains(message, "i/o timeout"), strings.Contains(message, "context deadline exceeded"):
		return "timed out; the host may be filtered or unreachable"
	case strings.Contains(message, "network is unreachable"):
		return "network unreachable from this App"
	}
	return "connection failed"
}

func collectStatus(
	ctx context.Context,
	optionsPath, metricsURL string,
	client *http.Client,
	dial func(context.Context, string, string) (net.Conn, error),
	now func() time.Time,
) statusResponse {
	response := statusResponse{
		AgentResponding: agentResponding(ctx, client, metricsURL),
		CheckedAt:       now().UTC().Format(time.RFC3339),
		Endpoints:       []endpointStatus{},
	}
	opts, err := loadOptions(optionsPath)
	if err == nil {
		response.Options = opts
	}

	endpoints := append([]string(nil), response.Options.AllowedEndpoints...)
	sort.Strings(endpoints)
	for _, endpoint := range endpoints {
		response.Endpoints = append(response.Endpoints, checkEndpoint(ctx, dial, endpoint))
	}
	return response
}
