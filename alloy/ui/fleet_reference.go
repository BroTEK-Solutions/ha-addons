package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type fleetReferenceRenderer interface {
	Render(context.Context, map[string]any) ([]byte, error)
}

type fleetReferenceManager interface {
	Issue(context.Context, map[string]any) (fleetReferenceIssue, error)
	Get(string) ([]byte, bool)
}

type commandFleetReferenceRenderer struct {
	generatorPath string
	validator     candidateValidator
	journalPath   string
}

func newCommandFleetReferenceRenderer(generatorPath string, validator candidateValidator) *commandFleetReferenceRenderer {
	return &commandFleetReferenceRenderer{
		generatorPath: generatorPath,
		validator:     validator,
		journalPath:   detectJournalPath(),
	}
}

func detectJournalPath() string {
	entries, err := os.ReadDir("/var/log/journal")
	if err == nil && len(entries) > 0 {
		return "/var/log/journal"
	}
	return "/run/log/journal"
}

func (r *commandFleetReferenceRenderer) Render(ctx context.Context, settings map[string]any) ([]byte, error) {
	if err := validateFleetReferenceSettings(settings); err != nil {
		return nil, err
	}

	// Validate the selected components as a Local candidate. The generated Fleet
	// pipeline uses the shared key, so supply it as each selected backend's
	// validation-only password without persisting duplicate secrets.
	candidate := make(map[string]any, len(settings)+4)
	for key, value := range settings {
		candidate[key] = value
	}
	candidate["operation_mode"] = "local"
	delete(candidate, "additional_config")
	sharedKey, _ := settings["gcloud_rw_api_key"].(string)
	for _, prefix := range []string{"loki", "prometheus", "tempo", "pyroscope"} {
		delete(candidate, prefix+"_password")
		if optionHasValue(candidate, prefix+"_username") {
			candidate[prefix+"_password"] = sharedKey
		}
	}
	if err := r.validator.Validate(ctx, candidate); err != nil {
		return nil, fmt.Errorf("validate Fleet starter pipeline: %w", err)
	}

	command := exec.CommandContext(ctx, "bash", r.generatorPath)
	command.Env = append(candidateEnvironment(settings),
		"FLEET_REFERENCE_PIPELINE=true",
		"JOURNAL_PATH="+r.journalPath,
	)
	var stderr strings.Builder
	command.Stderr = &stderr
	contents, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("render Fleet starter pipeline: %s", limitedMessage(stderr.String()))
	}
	return buildFleetManifest(settings, contents)
}

func validateFleetReferenceSettings(settings map[string]any) error {
	if stringSetting(settings, "operation_mode", "") != "fleet" {
		return errors.New("Fleet starter pipeline is available only in Fleet Management mode")
	}
	if optionEnabled(settings, "manual_config_enabled") {
		return errors.New("disable the full manual configuration override before generating a Fleet starter pipeline")
	}
	for _, endpoint := range []string{"loki_url", "prometheus_url", "tempo_url", "pyroscope_url"} {
		if optionHasValue(settings, endpoint) {
			return nil
		}
	}
	return errors.New("configure at least one telemetry destination before generating a Fleet starter pipeline")
}

type fleetReferenceIssue struct {
	Token     string
	ExpiresAt time.Time
}

type storedFleetReference struct {
	manifest  []byte
	expiresAt time.Time
}

type fleetReferenceBroker struct {
	renderer fleetReferenceRenderer
	random   io.Reader
	now      func() time.Time
	ttl      time.Duration

	mu         sync.Mutex
	references map[string]storedFleetReference
}

func newFleetReferenceBroker(renderer fleetReferenceRenderer, random io.Reader, now func() time.Time, ttl time.Duration) *fleetReferenceBroker {
	return &fleetReferenceBroker{
		renderer:   renderer,
		random:     random,
		now:        now,
		ttl:        ttl,
		references: make(map[string]storedFleetReference),
	}
}

func (b *fleetReferenceBroker) Issue(ctx context.Context, settings map[string]any) (fleetReferenceIssue, error) {
	manifest, err := b.renderer.Render(ctx, settings)
	if err != nil {
		return fleetReferenceIssue{}, err
	}
	tokenBytes := make([]byte, 24)
	if _, err := io.ReadFull(b.random, tokenBytes); err != nil {
		return fleetReferenceIssue{}, fmt.Errorf("create download token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := b.now().Add(b.ttl)

	b.mu.Lock()
	defer b.mu.Unlock()
	for key, reference := range b.references {
		if !reference.expiresAt.After(b.now()) {
			delete(b.references, key)
		}
	}
	b.references[token] = storedFleetReference{manifest: append([]byte(nil), manifest...), expiresAt: expiresAt}
	return fleetReferenceIssue{Token: token, ExpiresAt: expiresAt}, nil
}

func (b *fleetReferenceBroker) Get(token string) ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	reference, ok := b.references[token]
	if !ok || !reference.expiresAt.After(b.now()) {
		delete(b.references, token)
		return nil, false
	}
	return append([]byte(nil), reference.manifest...), true
}

func buildFleetManifest(settings map[string]any, contents []byte) ([]byte, error) {
	contents = []byte(strings.TrimSuffix(string(contents), "\n"))
	if len(strings.TrimSpace(string(contents))) == 0 {
		return nil, errors.New("Fleet starter pipeline has no enabled telemetry components")
	}
	instanceName := stringSetting(settings, "instance_name", "homeassistant")
	pipelineName := "home-assistant-" + fleetNameSlug(instanceName)
	if pipelineName == "home-assistant-" {
		return nil, errors.New("instance name must contain a letter or number")
	}

	var manifest strings.Builder
	fmt.Fprintf(&manifest, "apiVersion: fleet.ext.grafana.app/v1alpha1\nkind: Pipeline\nmetadata:\n  name: %s\nspec:\n  name: %s\n  enabled: true\n  contents: |-\n", pipelineName, pipelineName)
	for _, line := range strings.Split(string(contents), "\n") {
		fmt.Fprintf(&manifest, "    %s\n", line)
	}
	fmt.Fprintf(&manifest, "  matchers:\n    - %s\n", strconv.Quote("ha_addon_instance="+instanceName))
	return []byte(manifest.String()), nil
}

func fleetNameSlug(value string) string {
	var slug strings.Builder
	separator := false
	for _, character := range strings.ToLower(value) {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			if separator && slug.Len() > 0 {
				slug.WriteByte('-')
			}
			slug.WriteRune(character)
			separator = false
		} else {
			separator = true
		}
	}
	base := strings.Trim(slug.String(), "-")
	if base == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	suffix := fmt.Sprintf("%x", digest[:5])
	const pipelinePrefix = "home-assistant-"
	maxBaseLength := 63 - len(pipelinePrefix) - 1 - len(suffix)
	if len(base) > maxBaseLength {
		base = strings.TrimRight(base[:maxBaseLength], "-")
	}
	return base + "-" + suffix
}
