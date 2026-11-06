package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eclipse/paho.golang/packets"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A wire-level MQTT peer acknowledges actual client publications. It lets the
// test observe retained payloads and the last will without a Docker daemon.
func mqttPeer(t *testing.T) (int, <-chan packets.Packet) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan packets.Packet, 64)
	done := make(chan struct{})
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			t.Error("MQTT fixture did not stop")
		}
	})
	go func() {
		defer close(done)
		defer close(events)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		for {
			packet, err := packets.ReadPacket(conn)
			if err != nil {
				return
			}
			if _, ping := packet.Content.(*packets.Pingreq); !ping {
				events <- packet.Content
			}
			var reply packets.Packet
			switch p := packet.Content.(type) {
			case *packets.Connect:
				reply = &packets.Connack{Properties: &packets.Properties{}}
			case *packets.Publish:
				if p.QoS == 1 {
					reply = &packets.Puback{PacketID: p.PacketID, Properties: &packets.Properties{}}
				}
			case *packets.Pingreq:
				reply = &packets.Pingresp{}
			case *packets.Disconnect:
				return
			}
			if reply != nil {
				if _, err := reply.WriteTo(conn); err != nil {
					return
				}
			}
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, events
}

func startTestReporter(t *testing.T, app, enabled string, port int, credentials ...func() (string, string)) (chan error, context.CancelFunc) {
	t.Helper()
	// Keep run()'s real collection path, but redirect only its fixed local probe
	// addresses to a fixture rather than touching services on the test host.
	health := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("alloy_config_last_load_successful 1\n"))
	}))
	t.Cleanup(health.Close)
	baseTransport := http.DefaultTransport
	http.DefaultTransport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "127.0.0.1:12345", "127.0.0.1:4050", "127.0.0.1:8090":
			r = r.Clone(r.Context())
			r.URL.Host = health.Listener.Addr().String()
		}
		return baseTransport.RoundTrip(r)
	})
	t.Cleanup(func() { http.DefaultTransport = baseTransport })
	supervisor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/addons/self/info":
			_, _ = fmt.Fprintf(w, `{"result":"ok","data":{"slug":"repository_%s","name":"Custom display name","version":"1.0.0"}}`, app)
		case "/services/mqtt":
			username, password := "", ""
			if len(credentials) != 0 {
				username, password = credentials[0]()
			}
			_, _ = fmt.Fprintf(w, `{"result":"ok","data":{"host":"127.0.0.1","port":%d,"username":%q,"password":%q}}`, port, username, password)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(supervisor.Close)
	t.Setenv("SUPERVISOR_URL", supervisor.URL)
	t.Setenv("SUPERVISOR_TOKEN", "fixture-token")
	t.Setenv("REPORTER_APP", app)
	t.Setenv("REPORTER_MQTT_ENABLED", enabled)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Errorf("reporter stopped: %v", err)
			}
		case <-time.After(6 * time.Second):
			t.Error("reporter did not stop")
		}
	})
	return done, cancel
}

func nextMQTT(t *testing.T, events <-chan packets.Packet) packets.Packet {
	t.Helper()
	select {
	case p, ok := <-events:
		if !ok {
			t.Fatal("MQTT connection closed unexpectedly")
		}
		return p
	case <-time.After(4 * time.Second):
		t.Fatal("timed out waiting for MQTT packet")
		return nil
	}
}

func TestDisabledReporterWithdrawsRetainedTopicsAndIdles(t *testing.T) {
	port, events := mqttPeer(t)
	done, _ := startTestReporter(t, "alloy", "false", port)
	connect, ok := nextMQTT(t, events).(*packets.Connect)
	if !ok || connect.WillFlag {
		t.Fatal("cleanup connection must not leave a will that recreates retained availability")
	}
	want := map[string]bool{
		"homeassistant/binary_sensor/brotek_repository_alloy/ready/config":         false,
		"homeassistant/binary_sensor/brotek_repository_alloy/healthy/config":       false,
		"homeassistant/binary_sensor/brotek_repository_alloy/config_loaded/config": false,
		"homeassistant/sensor/brotek_repository_alloy/version/config":              false,
		"brotek/repository_alloy/state":                                            false,
		"brotek/repository_alloy/availability":                                     false,
	}
	for range len(want) {
		p, ok := nextMQTT(t, events).(*packets.Publish)
		if !ok {
			t.Fatal("expected retained deletion")
		}
		seen, exists := want[p.Topic]
		if !exists || seen || len(p.Payload) != 0 || !p.Retain || p.QoS != 1 {
			t.Fatalf("unexpected cleanup publication: topic=%s retain=%v qos=%d payload=%q", p.Topic, p.Retain, p.QoS, p.Payload)
		}
		want[p.Topic] = true
	}
	if _, ok := nextMQTT(t, events).(*packets.Disconnect); !ok {
		t.Fatal("cleanup must disconnect instead of polling health or publishing online")
	}
	select {
	case err := <-done:
		done <- err
		t.Fatal("disabled reporter must idle to avoid an s6 restart loop")
	default:
	}
}

func TestEnabledReporterPublishesReadableProductAndStableIdentity(t *testing.T) {
	for app, model := range map[string]string{
		"alloy": "Grafana Alloy", "grafana_pdc": "Grafana Private Data Source Connect",
		"grafana_sm": "Grafana Synthetic Monitoring Probe", "grafana_sm_browser": "Grafana Synthetic Monitoring Probe with Browser Checks",
	} {
		t.Run(app, func(t *testing.T) {
			port, events := mqttPeer(t)
			// An absent flag preserves pre-option installations.
			startTestReporter(t, app, "", port)
			connect, ok := nextMQTT(t, events).(*packets.Connect)
			if !ok || !connect.WillFlag {
				t.Fatal("enabled reporter must retain availability will")
			}
			p, ok := nextMQTT(t, events).(*packets.Publish)
			if !ok || !p.Retain {
				t.Fatal("expected retained discovery")
			}
			var payload discoveryPayload
			if err := json.Unmarshal(p.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Device.Model != model || payload.Device.Name != "Custom display name" {
				t.Errorf("product model or display name wrong: %+v", payload.Device)
			}
			if len(payload.Device.Identifiers) != 1 || payload.Device.Identifiers[0] != "brotek_repository_"+app {
				t.Errorf("device identity changed: %+v", payload.Device)
			}
			// Reach the running state before cancellation. Cancelling halfway
			// through discovery races Paho's connection teardown with Publish.
			for {
				p, ok := nextMQTT(t, events).(*packets.Publish)
				if !ok {
					t.Fatal("expected publication before first health state")
				}
				if p.Topic == "brotek/repository_"+app+"/state" {
					break
				}
			}
		})
	}
}

// Exercise two actual MQTT sessions, dropping the socket after the first state
// as a broker/network failure would. Credentials rotate before reconnection.
func TestReconnectRefreshesCredentialsAndRepublishesDiscovery(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan packets.Packet, 64)
	var rotated atomic.Bool
	done := make(chan struct{})
	t.Cleanup(func() { listener.Close(); <-done })
	go func() {
		defer close(done)
		defer close(events)
		for attempt := 0; attempt < 2; attempt++ {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			for {
				p, err := packets.ReadPacket(conn)
				if err != nil {
					break
				}
				if _, ping := p.Content.(*packets.Pingreq); !ping {
					events <- p.Content
				}
				var reply packets.Packet
				switch p := p.Content.(type) {
				case *packets.Pingreq:
					reply = &packets.Pingresp{}
				case *packets.Connect:
					reply = &packets.Connack{Properties: &packets.Properties{}}
				case *packets.Publish:
					if p.QoS == 1 {
						reply = &packets.Puback{PacketID: p.PacketID, Properties: &packets.Properties{}}
					}
				case *packets.Disconnect:
					conn.Close()
					return
				}
				if reply != nil {
					if _, err := reply.WriteTo(conn); err != nil {
						break
					}
				}
				if p, ok := p.Content.(*packets.Publish); ok && p.Topic == "brotek/repository_alloy/state" && attempt == 0 {
					rotated.Store(true)
					break
				}
			}
			conn.Close()
		}
	}()
	startTestReporter(t, "alloy", "true", listener.Addr().(*net.TCPAddr).Port, func() (string, string) {
		if rotated.Load() {
			return "new-user", "new-password"
		}
		return "old-user", "old-password"
	})
	for session := 0; session < 2; session++ {
		connect, ok := nextMQTT(t, events).(*packets.Connect)
		if !ok {
			t.Fatal("expected CONNECT")
		}
		expected := "old-user"
		if session == 1 {
			expected = "new-user"
		}
		expectedPassword := "old-password"
		if session == 1 {
			expectedPassword = "new-password"
		}
		if connect.Username != expected || string(connect.Password) != expectedPassword {
			t.Fatalf("session %d uses stale credentials", session)
		}
		discovery, online := 0, false
		for {
			packet := nextMQTT(t, events)
			p, ok := packet.(*packets.Publish)
			if !ok {
				t.Fatalf("expected publication, got %T: %+v", packet, packet)
			}
			if strings.HasSuffix(p.Topic, "/config") {
				discovery++
			}
			if p.Topic == "brotek/repository_alloy/availability" && string(p.Payload) == "online" {
				online = true
			}
			if p.Topic == "brotek/repository_alloy/state" {
				if discovery != 4 || !online {
					t.Fatalf("session %d missing discovery/online: %d %v", session, discovery, online)
				}
				break
			}
		}
	}
}
