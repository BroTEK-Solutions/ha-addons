// Command ha-reporter publishes an App's health to Home Assistant over MQTT.
//
// It is a side channel and must behave like one: it never touches the App's
// real work, and every failure mode here is "publish nothing" rather than
// "disturb the service". Home Assistant only learns about these entities when
// the user already runs an MQTT broker; without one the reporter idles quietly
// and keeps checking, so adding Mosquitto later needs no App restart.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

const (
	defaultInterval        = 30 * time.Second
	discoveryRetryInterval = 60 * time.Second
	probeTimeout           = 5 * time.Second
)

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func pollInterval() time.Duration {
	raw := os.Getenv("REPORTER_INTERVAL_SECONDS")
	if raw == "" {
		return defaultInterval
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 5 || seconds > 3600 {
		log.Printf("ignoring REPORTER_INTERVAL_SECONDS=%q; using %s", raw, defaultInterval)
		return defaultInterval
	}
	return time.Duration(seconds) * time.Second
}

// awaitBroker blocks until Home Assistant publishes an MQTT service. Having no
// broker is an ordinary state, not an error, so it is logged once rather than
// on every attempt.
func awaitBroker(ctx context.Context, supervisor *supervisorClient) (mqttService, error) {
	announced := false
	for {
		service, present, err := supervisor.MQTTService(ctx)
		switch {
		case err != nil:
			log.Printf("could not read the MQTT service from the Supervisor: %v", err)
		case present:
			return service, nil
		case !announced:
			log.Printf("no Home Assistant MQTT broker is configured; waiting to apply MQTT health reporting settings")
			announced = true
		}
		select {
		case <-ctx.Done():
			return mqttService{}, ctx.Err()
		case <-time.After(discoveryRetryInterval):
		}
	}
}

func run(ctx context.Context) error {
	enabled := true
	switch os.Getenv("REPORTER_MQTT_ENABLED") {
	case "", "true":
	case "false":
		enabled = false
	default:
		return errors.New("REPORTER_MQTT_ENABLED must be true or false")
	}
	app := envOr("REPORTER_APP", "")
	if app == "" {
		return errors.New("REPORTER_APP is required")
	}
	entities, err := specsFor(app)
	if err != nil {
		return err
	}
	token := os.Getenv("SUPERVISOR_TOKEN")
	if token == "" {
		return errors.New("SUPERVISOR_TOKEN is required")
	}

	supervisor := newSupervisorClient(
		envOr("SUPERVISOR_URL", "http://supervisor"),
		token,
		&http.Client{Timeout: 15 * time.Second},
	)

	// Never guess the registry identity: a fallback slug could create duplicate
	// devices or leave the real discovery topics behind when disabling reporting.
	info, err := awaitAppInfo(ctx, supervisor)
	if err != nil {
		return err
	}
	info.Slug = sanitizeSlug(info.Slug)
	info.Model = modelFor(app)

	service, err := awaitBroker(ctx, supervisor)
	if err != nil {
		return err
	}

	brokerURL, err := url.Parse(service.Address())
	if err != nil {
		return err
	}
	names := topicsFor(info.Slug)

	// The retained will message is what marks every entity unavailable if this
	// reporter dies, rather than leaving Home Assistant showing a stale value
	// forever.
	config := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{brokerURL},
		KeepAlive:                     30,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         60,
		ConnectUsername:               service.Username,
		ConnectPassword:               []byte(service.Password),
		WillMessage: &paho.WillMessage{
			Topic:   names.availability,
			Payload: []byte(payloadOffline),
			QoS:     1,
			Retain:  true,
		},
		OnConnectError: func(err error) { log.Printf("MQTT connection attempt failed: %v", err) },
		ClientConfig: paho.ClientConfig{
			ClientID: topicRoot + "-" + info.Slug,
		},
	}
	if !enabled {
		// A cleanup connection must not recreate availability after withdrawal.
		config.WillMessage = nil
	}

	connection, err := autopaho.NewConnection(ctx, config)
	if err != nil {
		return err
	}
	if err := connection.AwaitConnection(ctx); err != nil {
		return err
	}
	if !enabled {
		for {
			cleanup, cancel := context.WithTimeout(ctx, 15*time.Second)
			err := withdrawDiscovery(cleanup, connection, entities, info.Slug)
			cancel()
			if err == nil {
				break
			}
			log.Printf("MQTT health reporting is disabled; discovery cleanup will retry: %v", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(discoveryRetryInterval):
			}
		}
		shutdown, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := connection.Disconnect(shutdown)
		cancel()
		if err != nil {
			return err
		}
		log.Printf("MQTT health reporting is disabled; retained discovery removed")
		// s6 supervises this process in Alloy/PDC. Exiting successfully would
		// restart it and repeat cleanup forever. No broker or health polling now.
		<-ctx.Done()
		return ctx.Err()
	}
	log.Printf("publishing %d Home Assistant entities for %s over MQTT", len(entities), info.Slug)

	// Discovery messages are retained so Home Assistant rebuilds the entities
	// after its own restart without waiting for this reporter to republish.
	for _, entity := range entities {
		payload, err := buildDiscovery(entity, info)
		if err != nil {
			return err
		}
		if _, err := connection.Publish(ctx, &paho.Publish{
			Topic:   discoveryTopic(entity, info.Slug),
			QoS:     1,
			Retain:  true,
			Payload: payload,
		}); err != nil {
			return err
		}
	}

	if _, err := connection.Publish(ctx, &paho.Publish{
		Topic:   names.availability,
		QoS:     1,
		Retain:  true,
		Payload: []byte(payloadOnline),
	}); err != nil {
		return err
	}

	collector := newCollector(&http.Client{Timeout: probeTimeout})
	ticker := time.NewTicker(pollInterval())
	defer ticker.Stop()
	for {
		state, err := buildState(collector.Collect(ctx, entities))
		if err != nil {
			log.Printf("could not encode the state payload: %v", err)
		} else if _, err := connection.Publish(ctx, &paho.Publish{
			Topic:   names.state,
			QoS:     0,
			Retain:  true,
			Payload: state,
		}); err != nil && ctx.Err() == nil {
			log.Printf("could not publish state: %v", err)
		}

		select {
		case <-ctx.Done():
			// Mark the entities unavailable on an orderly shutdown; the will
			// message only covers the disorderly case.
			shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			_, _ = connection.Publish(shutdown, &paho.Publish{
				Topic:   names.availability,
				QoS:     1,
				Retain:  true,
				Payload: []byte(payloadOffline),
			})
			_ = connection.Disconnect(shutdown)
			cancel()
			return nil
		case <-ticker.C:
		}
	}
}

func awaitAppInfo(ctx context.Context, supervisor *supervisorClient) (appInfo, error) {
	for {
		info, err := supervisor.AppInfo(ctx)
		if err == nil && info.Slug != "" {
			return info, nil
		}
		log.Printf("waiting for App identity from the Supervisor (error: %v)", err)
		select {
		case <-ctx.Done():
			return appInfo{}, ctx.Err()
		case <-time.After(discoveryRetryInterval):
		}
	}
}

// Empty retained publications remove only this App's existing discovery and
// state. QoS 1 acknowledgement is required before treating cleanup as complete.
func withdrawDiscovery(ctx context.Context, connection *autopaho.ConnectionManager, entities []entitySpec, slug string) error {
	topics := make([]string, 0, len(entities)+2)
	for _, entity := range entities {
		topics = append(topics, discoveryTopic(entity, slug))
	}
	names := topicsFor(slug)
	topics = append(topics, names.state, names.availability)
	for _, topic := range topics {
		response, err := connection.Publish(ctx, &paho.Publish{Topic: topic, QoS: 1, Retain: true})
		if err != nil {
			return err
		}
		if response == nil || response.ReasonCode >= 0x80 {
			return fmt.Errorf("broker did not acknowledge removal of %s", topic)
		}
	}
	return nil
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("ha-reporter: ")
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Printf("stopping: %v", err)
		os.Exit(1)
	}
}
