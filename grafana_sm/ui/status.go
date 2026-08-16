package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// options deliberately omits api_token. This value is serialized to the
// browser, so not decoding the secret makes leaking it structurally impossible.
type options struct {
	APIServerAddress     string `json:"api_server_address"`
	LogLevel             string `json:"log_level"`
	AllowPrivateNetworks bool   `json:"allow_private_networks"`
	DisableUsageReports  bool   `json:"disable_usage_reports"`
}

type checkMetrics struct {
	Type       string  `json:"type"`
	Running    float64 `json:"running"`
	Executions float64 `json:"executions"`
	Errors     float64 `json:"errors"`
}

type diagnostics struct {
	Connected         bool           `json:"connected"`
	AgentVersion      string         `json:"agent_version,omitempty"`
	ProcessUptime     float64        `json:"process_uptime_seconds,omitempty"`
	PublisherHandlers float64        `json:"publisher_handlers"`
	Pushes            float64        `json:"pushes"`
	PushErrors        float64        `json:"push_errors"`
	PushFailures      float64        `json:"push_failures"`
	Retries           float64        `json:"retries"`
	Dropped           float64        `json:"dropped"`
	Checks            []checkMetrics `json:"checks"`
}

type endpointStatus struct {
	Reachable bool   `json:"reachable"`
	Detail    string `json:"detail"`
}

type statusResponse struct {
	AgentResponding bool           `json:"agent_responding"`
	BrowserChecks   bool           `json:"browser_checks"`
	Options         options        `json:"options"`
	Endpoint        endpointStatus `json:"endpoint"`
	Diagnostics     diagnostics    `json:"diagnostics"`
	CheckedAt       string         `json:"checked_at"`
}

type sample struct {
	name   string
	labels map[string]string
	value  float64
}

func loadOptions(path string) (options, error) {
	if safe := os.Getenv("SM_UI_OPTIONS"); safe != "" {
		var result options
		if err := json.Unmarshal([]byte(safe), &result); err != nil {
			return options{}, err
		}
		return result, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return options{}, err
	}
	defer file.Close()
	var result options
	err = json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&result)
	return result, err
}

// parseMetrics retains only scalar samples and labels needed by the explicit
// diagnostics allowlist below. Arbitrary upstream labels never reach JSON.
func parseMetrics(reader io.Reader) []sample {
	var result []sample
	scanner := bufio.NewScanner(io.LimitReader(reader, 16<<20))
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, labels, rest := line, map[string]string{}, ""
		if open := strings.IndexByte(line, '{'); open >= 0 {
			close := strings.IndexByte(line[open:], '}')
			if close < 0 {
				continue
			}
			close += open
			name, rest = line[:open], strings.TrimSpace(line[close+1:])
			for _, field := range strings.Split(line[open+1:close], ",") {
				parts := strings.SplitN(field, "=", 2)
				if len(parts) == 2 {
					labels[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), `"`)
				}
			}
		} else if space := strings.IndexAny(line, " \t"); space >= 0 {
			name, rest = line[:space], strings.TrimSpace(line[space:])
		} else {
			continue
		}
		if space := strings.IndexAny(rest, " \t"); space >= 0 {
			rest = rest[:space]
		}
		value, err := strconv.ParseFloat(rest, 64)
		if err == nil {
			result = append(result, sample{name: name, labels: labels, value: value})
		}
	}
	return result
}

var safeCheckTypes = map[string]string{
	"browser": "Browser", "dns": "DNS", "http": "HTTP", "k6": "Browser",
	"multihttp": "MultiHTTP", "ping": "Ping", "traceroute": "Traceroute",
}

func summarizeMetrics(samples []sample, now time.Time) diagnostics {
	result := diagnostics{Checks: []checkMetrics{}}
	checks := map[string]*checkMetrics{}
	for _, item := range samples {
		switch item.name {
		case "sm_agent_api_connection_status":
			result.Connected = item.value != 0
		case "sm_agent_info":
			result.AgentVersion = item.labels["version"]
		case "process_start_time_seconds":
			result.ProcessUptime = float64(max(0, now.Unix()-int64(item.value)))
		case "sm_agent_publisher_handlers_total":
			result.PublisherHandlers += item.value
		case "sm_agent_publisher_push_total":
			result.Pushes += item.value
		case "sm_agent_publisher_push_errors_total":
			result.PushErrors += item.value
		case "sm_agent_publisher_push_failed_total":
			result.PushFailures += item.value
		case "sm_agent_publisher_retries_total":
			result.Retries += item.value
		case "sm_agent_publisher_drop_total":
			result.Dropped += item.value
		case "sm_agent_updater_scrapers_total", "sm_agent_scraper_operations_total", "sm_agent_scraper_errors_total":
			display, ok := safeCheckTypes[strings.ToLower(item.labels["type"])]
			if !ok {
				continue
			}
			entry := checks[display]
			if entry == nil {
				entry = &checkMetrics{Type: display}
				checks[display] = entry
			}
			switch item.name {
			case "sm_agent_updater_scrapers_total":
				entry.Running += item.value
			case "sm_agent_scraper_operations_total":
				entry.Executions += item.value
			case "sm_agent_scraper_errors_total":
				entry.Errors += item.value
			}
		}
	}
	for _, item := range checks {
		result.Checks = append(result.Checks, *item)
	}
	sort.Slice(result.Checks, func(i, j int) bool { return result.Checks[i].Type < result.Checks[j].Type })
	return result
}

func fetchMetrics(ctx context.Context, client *http.Client, url string, now time.Time) (diagnostics, bool) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return diagnostics{Checks: []checkMetrics{}}, false
	}
	response, err := client.Do(request)
	if err != nil {
		return diagnostics{Checks: []checkMetrics{}}, false
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return diagnostics{Checks: []checkMetrics{}}, false
	}
	return summarizeMetrics(parseMetrics(response.Body), now), true
}

func checkEndpoint(ctx context.Context, address string, dial func(context.Context, string, string) (net.Conn, error)) endpointStatus {
	attempt, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	connection, err := dial(attempt, "tcp", address)
	if err != nil {
		message := "connection failed"
		switch text := err.Error(); {
		case strings.Contains(text, "no such host"):
			message = "hostname does not resolve"
		case strings.Contains(text, "connection refused"):
			message = "host reachable, but the port refused the connection"
		case strings.Contains(text, "timeout"), strings.Contains(text, "deadline exceeded"):
			message = "connection timed out"
		}
		return endpointStatus{Detail: message}
	}
	_ = connection.Close()
	return endpointStatus{Reachable: true, Detail: "TCP connection succeeded"}
}

func collectStatus(ctx context.Context, path, metricsURL string, browser bool, client *http.Client, dial func(context.Context, string, string) (net.Conn, error), now func() time.Time) statusResponse {
	checked := now().UTC()
	result := statusResponse{BrowserChecks: browser, CheckedAt: checked.Format(time.RFC3339)}
	if opts, err := loadOptions(path); err == nil {
		result.Options = opts
		result.Endpoint = checkEndpoint(ctx, opts.APIServerAddress, dial)
	}
	result.Diagnostics, result.AgentResponding = fetchMetrics(ctx, client, metricsURL, checked)
	return result
}
