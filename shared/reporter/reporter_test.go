package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseMetricsReadsNamesLabelsAndValues(t *testing.T) {
	input := strings.Join([]string{
		"# HELP alloy_build_info Build information",
		"# TYPE alloy_build_info gauge",
		`alloy_build_info{version="1.18.1",goversion="go1.26"} 1`,
		"alloy_config_last_load_successful 1",
		`http_requests_total{code="200",path="/a,b"} 42 1700000000000`,
		"malformed_line_without_value",
		`bad_value{a="b"} not-a-number`,
		"",
	}, "\n")

	samples := parseMetrics(strings.NewReader(input))
	if len(samples) != 3 {
		t.Fatalf("expected 3 usable samples, got %d (%+v)", len(samples), samples)
	}
	if samples[0].Name != "alloy_build_info" || samples[0].Labels["version"] != "1.18.1" {
		t.Errorf("build info parsed incorrectly: %+v", samples[0])
	}
	if samples[1].Name != "alloy_config_last_load_successful" || samples[1].Value != 1 {
		t.Errorf("bare series parsed incorrectly: %+v", samples[1])
	}
	// A comma inside a quoted label value must not split the label set, and a
	// trailing timestamp must not be read as the value.
	if samples[2].Labels["path"] != "/a,b" || samples[2].Value != 42 {
		t.Errorf("quoted label or timestamp handled incorrectly: %+v", samples[2])
	}
}

func TestParseMetricsHandlesEscapedLabelValues(t *testing.T) {
	samples := parseMetrics(strings.NewReader(`x{a="say \"hi\"",b="line\nbreak"} 1`))
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].Labels["a"] != `say "hi"` {
		t.Errorf("escaped quote not decoded: %q", samples[0].Labels["a"])
	}
	if samples[0].Labels["b"] != "line\nbreak" {
		t.Errorf("escaped newline not decoded: %q", samples[0].Labels["b"])
	}
}

func TestResolveMetricReportsUnknownWhenSeriesIsAbsent(t *testing.T) {
	entity := entitySpec{
		Component: componentBinarySensor,
		Probe:     probe{Kind: probeMetric, Metric: "alloy_config_last_load_successful"},
	}
	// An upstream rename must not be reported as a confident failure state.
	if value := resolveMetric(entity, nil); value != nil {
		t.Fatalf("absent series must resolve to unknown, got %v", value)
	}
	present := []sample{{Name: "alloy_config_last_load_successful", Value: 1}}
	if value := resolveMetric(entity, present); value != "ON" {
		t.Fatalf("expected ON, got %v", value)
	}
	failing := []sample{{Name: "alloy_config_last_load_successful", Value: 0}}
	if value := resolveMetric(entity, failing); value != "OFF" {
		t.Fatalf("expected OFF, got %v", value)
	}
}

func TestResolveMetricReadsLabelValues(t *testing.T) {
	entity := entitySpec{
		Component: componentSensor,
		Probe:     probe{Kind: probeMetricLabel, Metric: "alloy_build_info", Label: "version"},
	}
	samples := []sample{{Name: "alloy_build_info", Labels: map[string]string{"version": "1.18.1"}, Value: 1}}
	if value := resolveMetric(entity, samples); value != "1.18.1" {
		t.Fatalf("expected the version label, got %v", value)
	}
	missingLabel := []sample{{Name: "alloy_build_info", Labels: map[string]string{}, Value: 1}}
	if value := resolveMetric(entity, missingLabel); value != nil {
		t.Fatalf("absent label must resolve to unknown, got %v", value)
	}
}

func TestCollectScrapesEachEndpointOncePerCycle(t *testing.T) {
	var scrapes int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			scrapes++
			_, _ = w.Write([]byte("alloy_config_last_load_successful 1\n" +
				`alloy_build_info{version="1.18.1"} 1` + "\n"))
		case "/-/ready":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	entities := []entitySpec{
		{Key: "ready", Component: componentBinarySensor, Probe: probe{Kind: probeHTTPStatus, URL: server.URL + "/-/ready"}},
		{Key: "healthy", Component: componentBinarySensor, Probe: probe{Kind: probeHTTPStatus, URL: server.URL + "/-/healthy"}},
		{Key: "config_loaded", Component: componentBinarySensor, Probe: probe{Kind: probeMetric, URL: server.URL + "/metrics", Metric: "alloy_config_last_load_successful"}},
		{Key: "version", Component: componentSensor, Probe: probe{Kind: probeMetricLabel, URL: server.URL + "/metrics", Metric: "alloy_build_info", Label: "version"}},
	}

	state := newCollector(&http.Client{Timeout: 2 * time.Second}).Collect(context.Background(), entities)

	if scrapes != 1 {
		t.Errorf("two entities share one endpoint, so it must be scraped once; got %d", scrapes)
	}
	if state["ready"] != "ON" {
		t.Errorf("ready should be ON, got %v", state["ready"])
	}
	// A 503 is a real observation that the endpoint is not healthy, unlike a
	// missing metric, so it is OFF rather than unknown.
	if state["healthy"] != "OFF" {
		t.Errorf("healthy should be OFF, got %v", state["healthy"])
	}
	if state["version"] != "1.18.1" {
		t.Errorf("version should come from the label, got %v", state["version"])
	}
}

func TestBuildStateOmitsUnknownValues(t *testing.T) {
	encoded, err := buildState(map[string]any{"ready": "ON", "version": nil})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, present := decoded["version"]; present {
		t.Error("an unknown value must be omitted so the template falls through to its default")
	}
	if decoded["ready"] != "ON" {
		t.Errorf("known values must be retained, got %v", decoded["ready"])
	}
}

func TestBuildDiscoveryDescribesTheEntityAndItsDevice(t *testing.T) {
	info := appInfo{Slug: "alloy", Name: "Grafana Alloy", Version: "2.3.0"}
	entity := entitySpec{
		Key: "ready", Name: "Ready", Component: componentBinarySensor, DeviceClass: "running",
	}
	encoded, err := buildDiscovery(entity, info)
	if err != nil {
		t.Fatal(err)
	}
	var payload discoveryPayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.UniqueID != "brotek_alloy_ready" {
		t.Errorf("unique id must be stable across restarts, got %q", payload.UniqueID)
	}
	if payload.StateTopic != "brotek/alloy/state" {
		t.Errorf("unexpected state topic %q", payload.StateTopic)
	}
	if !strings.Contains(payload.ValueTemplate, "value_json.ready") {
		t.Errorf("value template must select this entity's key, got %q", payload.ValueTemplate)
	}
	if payload.AvailabilityTopic != "brotek/alloy/availability" {
		t.Errorf("unexpected availability topic %q", payload.AvailabilityTopic)
	}
	if len(payload.Device.Identifiers) != 1 || payload.Device.Identifiers[0] != "brotek_alloy" {
		t.Errorf("entities must group under one device, got %+v", payload.Device.Identifiers)
	}
	if payload.Device.SWVersion != "2.3.0" {
		t.Errorf("device should carry the App version, got %q", payload.Device.SWVersion)
	}
}

func TestDiscoveryTopicUsesTheComponentAndKey(t *testing.T) {
	topic := discoveryTopic(entitySpec{Key: "ready", Component: componentBinarySensor}, "alloy")
	if topic != "homeassistant/binary_sensor/brotek_alloy/ready/config" {
		t.Fatalf("unexpected discovery topic %q", topic)
	}
}

func TestSpecsAreDefinedForEveryPublishedApp(t *testing.T) {
	for _, app := range []string{"alloy", "grafana_pdc", "grafana_sm", "grafana_sm_browser"} {
		entities, err := specsFor(app)
		if err != nil {
			t.Fatalf("%s: %v", app, err)
		}
		if len(entities) == 0 {
			t.Fatalf("%s: no entities defined", app)
		}
		seen := map[string]bool{}
		for _, entity := range entities {
			if entity.Key == "" || entity.Name == "" || entity.Component == "" {
				t.Errorf("%s: incomplete entity %+v", app, entity)
			}
			if seen[entity.Key] {
				t.Errorf("%s: duplicate entity key %q would collide in the state payload", app, entity.Key)
			}
			seen[entity.Key] = true
		}
	}
	if _, err := specsFor("unknown_app"); err == nil {
		t.Error("an unknown App must be rejected rather than publishing nothing silently")
	}
}

func TestMQTTServiceTreatsAnAbsentBrokerAsAnOrdinaryState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"result":"error","message":"no such service"}`))
	}))
	defer server.Close()

	_, present, err := newSupervisorClient(server.URL, "token", server.Client()).
		MQTTService(context.Background())
	if err != nil {
		t.Fatalf("an absent broker must not be an error: %v", err)
	}
	if present {
		t.Error("no broker should be reported as absent")
	}
}

func TestMQTTServiceReturnsBrokerDetails(t *testing.T) {
	var authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"result":"ok","data":{"host":"core-mosquitto","port":1883,` +
			`"username":"addons","password":"secret","ssl":false}}`))
	}))
	defer server.Close()

	service, present, err := newSupervisorClient(server.URL, "token", server.Client()).
		MQTTService(context.Background())
	if err != nil || !present {
		t.Fatalf("expected a broker, got present=%v err=%v", present, err)
	}
	if authorization != "Bearer token" {
		t.Errorf("the Supervisor token must be sent, got %q", authorization)
	}
	if service.Address() != "mqtt://core-mosquitto:1883" {
		t.Errorf("unexpected broker address %q", service.Address())
	}
	service.SSL = true
	if service.Address() != "tls://core-mosquitto:1883" {
		t.Errorf("TLS brokers must use the tls scheme, got %q", service.Address())
	}
}

func TestSanitizeSlugKeepsTopicsWellFormed(t *testing.T) {
	for input, expected := range map[string]string{
		"alloy":            "alloy",
		"a141124a_alloy":   "a141124a_alloy",
		"weird/slug#chars": "weird_slug_chars",
		"":                 "app",
	} {
		if actual := sanitizeSlug(input); actual != expected {
			t.Errorf("sanitizeSlug(%q) = %q, want %q", input, actual, expected)
		}
	}
}
