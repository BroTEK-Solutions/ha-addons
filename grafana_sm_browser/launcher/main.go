package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	agentPath   = "/usr/local/bin/synthetic-monitoring-agent"
	optionsPath = "/data/options.json"
	healthURL   = "http://127.0.0.1:4050/"
)

type options struct {
	APIToken             string `json:"api_token"`
	APIServerAddress     string `json:"api_server_address"`
	LogLevel             string `json:"log_level"`
	AllowPrivateNetworks bool   `json:"allow_private_networks"`
	DisableUsageReports  bool   `json:"disable_usage_reports"`
}

type agentProcess struct {
	Args []string
	Env  []string
}

func loadOptions(reader io.Reader) (options, error) {
	result := options{LogLevel: "warn"}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&result); err != nil {
		return options{}, fmt.Errorf("read App options: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return options{}, errors.New("read App options: multiple JSON values")
		}
		return options{}, fmt.Errorf("read App options: %w", err)
	}
	return result, nil
}

func buildAgentProcess(opts options, environment []string) (agentProcess, error) {
	if opts.APIToken == "" {
		return agentProcess{}, errors.New("api_token is required")
	}
	if strings.Contains(opts.APIServerAddress, "://") {
		return agentProcess{}, errors.New("api_server_address must be a host:port without a URL scheme")
	}
	host, portText, err := net.SplitHostPort(opts.APIServerAddress)
	if err != nil || host == "" {
		return agentProcess{}, errors.New("api_server_address must be a host:port without a URL scheme")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return agentProcess{}, errors.New("api_server_address must use a valid TCP port")
	}

	args := []string{
		agentPath,
		"--api-server-address=" + opts.APIServerAddress,
		"--listen-address=127.0.0.1:4050",
	}
	switch opts.LogLevel {
	case "warn":
	case "info":
		args = append(args, "--verbose=true")
	case "debug":
		args = append(args, "--debug=true")
	default:
		return agentProcess{}, fmt.Errorf("log_level %q is not supported", opts.LogLevel)
	}
	if opts.AllowPrivateNetworks {
		args = append(args, "--blocked-nets=")
	}
	if opts.DisableUsageReports {
		args = append(args, "--disable-usage-reports=true")
	}
	return agentProcess{
		Args: args,
		Env:  setEnvironment(environment, "SM_AGENT_API_TOKEN", opts.APIToken),
	}, nil
}

func setEnvironment(environment []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if !strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return append(result, prefix+value)
}

func checkHealth(client *http.Client, endpoint string) error {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("liveness endpoint returned HTTP %d", response.StatusCode)
	}
	return nil
}

func writeFatal(writer io.Writer, secret, message string) {
	if secret != "" {
		message = strings.ReplaceAll(message, secret, "[redacted]")
	}
	fmt.Fprintf(writer, "Fatal: %s\n", message)
}

func run() error {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		return checkHealth(&http.Client{Timeout: 5 * time.Second}, healthURL)
	}
	file, err := os.Open(optionsPath)
	if err != nil {
		return fmt.Errorf("open App options: %w", err)
	}
	defer file.Close()

	opts, err := loadOptions(file)
	if err != nil {
		return err
	}
	process, err := buildAgentProcess(opts, os.Environ())
	if err != nil {
		return err
	}
	if err := syscall.Exec(agentPath, process.Args, process.Env); err != nil {
		return fmt.Errorf("start Synthetic Monitoring Agent: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		secret := ""
		if file, openErr := os.Open(optionsPath); openErr == nil {
			if opts, decodeErr := loadOptions(file); decodeErr == nil {
				secret = opts.APIToken
			}
			file.Close()
		}
		writeFatal(os.Stderr, secret, err.Error())
		os.Exit(1)
	}
}
