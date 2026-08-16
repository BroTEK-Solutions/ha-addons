package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	agentPath    = "/usr/local/bin/synthetic-monitoring-agent"
	reporterPath = "/usr/local/bin/ha-reporter"
	uiPath       = "/usr/local/bin/ha-sm-ui"
	agentUID     = 12345
	agentGID     = 12345
	optionsPath  = "/data/options.json"
	healthURL    = "http://127.0.0.1:4050/"
)

type options struct {
	APIToken             string `json:"api_token"`
	APIServerAddress     string `json:"api_server_address"`
	LogLevel             string `json:"log_level"`
	AllowPrivateNetworks bool   `json:"allow_private_networks"`
	DisableUsageReports  bool   `json:"disable_usage_reports"`
}

type agentProcess struct {
	Path string
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
		Path: agentPath,
		Args: args,
		Env:  setEnvironment(environment, "SM_AGENT_API_TOKEN", opts.APIToken),
	}, nil
}

func wrapWithTini(process agentProcess, tiniPath string) agentProcess {
	args := make([]string, 0, len(process.Args)+2)
	args = append(args, tiniPath, "--")
	args = append(args, process.Args...)
	process.Path = tiniPath
	process.Args = args
	return process
}

// startReporter launches the optional Home Assistant MQTT reporter alongside
// the agent. This App has no supervision tree - the launcher execs the agent and
// is replaced by it - so the reporter is started as a child beforehand and is
// bounded by the container's lifetime.
//
// Every failure here is deliberately silent-ish and non-fatal: the probe is the
// product, and a telemetry side channel must never keep it from starting. The
// child is started while the launcher is still root so it can be dropped to the
// same unprivileged account the agent runs as, and it inherits only the ambient
// environment, never the probe's API token.
func startReporter(reporterPath string, environment []string, log io.Writer) {
	if os.Getenv("REPORTER_APP") == "" {
		return
	}
	if _, err := os.Stat(reporterPath); err != nil {
		return
	}
	command := exec.Command(reporterPath)
	command.Env = environment
	command.Stdout = log
	command.Stderr = log
	command.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: agentUID, Gid: agentGID},
	}
	if err := command.Start(); err != nil {
		fmt.Fprintf(log, "Warning: could not start the Home Assistant MQTT reporter: %v\n", err)
	}
}

// startUI launches the ingress-only, read-only status server before this
// launcher is replaced by the agent. Like the reporter, it is deliberately a
// sibling process bounded by the container lifetime and receives no API token.
func startUI(path string, environment []string, opts options, log io.Writer) {
	if _, err := os.Stat(path); err != nil {
		return
	}
	safeOptions, err := json.Marshal(struct {
		APIServerAddress     string `json:"api_server_address"`
		LogLevel             string `json:"log_level"`
		AllowPrivateNetworks bool   `json:"allow_private_networks"`
		DisableUsageReports  bool   `json:"disable_usage_reports"`
	}{opts.APIServerAddress, opts.LogLevel, opts.AllowPrivateNetworks, opts.DisableUsageReports})
	if err != nil {
		fmt.Fprintf(log, "Warning: could not prepare ingress status options: %v\n", err)
		return
	}
	command := exec.Command(path)
	command.Env = setEnvironment(environment, "SM_UI_OPTIONS", string(safeOptions))
	command.Stdout = log
	command.Stderr = log
	command.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{Uid: agentUID, Gid: agentGID},
	}
	if err := command.Start(); err != nil {
		fmt.Fprintf(log, "Warning: could not start the ingress status page: %v\n", err)
	}
}

func dropPrivileges() error {
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("clear supplementary groups: %w", err)
	}
	if err := syscall.Setgid(agentGID); err != nil {
		return fmt.Errorf("set agent group: %w", err)
	}
	if err := syscall.Setuid(agentUID); err != nil {
		return fmt.Errorf("set agent user: %w", err)
	}
	return nil
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
	withTini := false
	switch {
	case len(os.Args) == 1:
	case len(os.Args) == 2 && os.Args[1] == "--with-tini":
		withTini = true
	default:
		return errors.New("launcher accepts only --with-tini or healthcheck")
	}
	if os.Geteuid() != 0 {
		return errors.New("configuration launcher must start as root")
	}
	file, err := os.Open(optionsPath)
	if err != nil {
		return fmt.Errorf("open App options: %w", err)
	}
	opts, err := loadOptions(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return fmt.Errorf("close App options: %w", closeErr)
	}
	process, err := buildAgentProcess(opts, os.Environ())
	if err != nil {
		return err
	}
	if withTini {
		tiniPath, err := exec.LookPath("tini")
		if err != nil {
			return fmt.Errorf("find tini: %w", err)
		}
		process = wrapWithTini(process, tiniPath)
	}
	// Started before the privilege drop, because setting the child's credentials
	// requires the launcher to still be root. os.Environ() is passed rather than
	// the agent's environment so the probe's API token never reaches it.
	startReporter(reporterPath, os.Environ(), os.Stderr)
	startUI(uiPath, os.Environ(), opts, os.Stderr)
	if err := dropPrivileges(); err != nil {
		return fmt.Errorf("drop launcher privileges: %w", err)
	}
	if err := syscall.Exec(process.Path, process.Args, process.Env); err != nil {
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
