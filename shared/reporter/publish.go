package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	discoveryPrefix = "homeassistant"
	topicRoot       = "brotek"
	payloadOnline   = "online"
	payloadOffline  = "offline"
)

// device is the Home Assistant device registry entry every entity attaches to,
// so the App's entities group under one device instead of arriving as orphans.
type device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model,omitempty"`
	SWVersion    string   `json:"sw_version,omitempty"`
}

type discoveryPayload struct {
	Name              string `json:"name"`
	UniqueID          string `json:"unique_id"`
	StateTopic        string `json:"state_topic"`
	ValueTemplate     string `json:"value_template"`
	AvailabilityTopic string `json:"availability_topic"`
	PayloadAvailable  string `json:"payload_available"`
	PayloadNotAvail   string `json:"payload_not_available"`
	DeviceClass       string `json:"device_class,omitempty"`
	EntityCategory    string `json:"entity_category,omitempty"`
	Icon              string `json:"icon,omitempty"`
	Device            device `json:"device"`
}

type topics struct {
	state        string
	availability string
}

func topicsFor(slug string) topics {
	return topics{
		state:        fmt.Sprintf("%s/%s/state", topicRoot, slug),
		availability: fmt.Sprintf("%s/%s/availability", topicRoot, slug),
	}
}

func deviceFor(info appInfo) device {
	name := info.Name
	if name == "" {
		name = info.Slug
	}
	return device{
		Identifiers:  []string{topicRoot + "_" + info.Slug},
		Name:         name,
		Manufacturer: "BroTEK Solutions",
		Model:        info.Slug,
		SWVersion:    info.Version,
	}
}

// discoveryTopic addresses one entity's retained configuration message.
func discoveryTopic(entity entitySpec, slug string) string {
	return fmt.Sprintf("%s/%s/%s/%s/config", discoveryPrefix, entity.Component, topicRoot+"_"+slug, entity.Key)
}

// buildDiscovery renders the retained configuration message for one entity.
// Every entity reads the same JSON state topic through its own value template,
// which keeps one published message per cycle regardless of entity count.
func buildDiscovery(entity entitySpec, info appInfo) ([]byte, error) {
	slug := info.Slug
	names := topicsFor(slug)
	payload := discoveryPayload{
		Name:              entity.Name,
		UniqueID:          fmt.Sprintf("%s_%s_%s", topicRoot, slug, entity.Key),
		StateTopic:        names.state,
		ValueTemplate:     fmt.Sprintf("{{ value_json.%s | default('') }}", entity.Key),
		AvailabilityTopic: names.availability,
		PayloadAvailable:  payloadOnline,
		PayloadNotAvail:   payloadOffline,
		DeviceClass:       entity.DeviceClass,
		EntityCategory:    entity.EntityCategory,
		Icon:              entity.Icon,
		Device:            deviceFor(info),
	}
	return json.Marshal(payload)
}

// buildState renders the single JSON document every entity reads. Keys whose
// value is unknown are omitted rather than sent as null, because an absent key
// makes the template fall through to its default and Home Assistant shows the
// entity as unknown - which is what an unobservable state should look like.
func buildState(values map[string]any) ([]byte, error) {
	present := make(map[string]any, len(values))
	for key, value := range values {
		if value == nil {
			continue
		}
		present[key] = value
	}
	return json.Marshal(present)
}

// sanitizeSlug keeps the slug usable inside an MQTT topic. Supervisor slugs are
// already restricted, so this only guards against a surprise.
func sanitizeSlug(slug string) string {
	replaced := strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z',
			character >= 'A' && character <= 'Z',
			character >= '0' && character <= '9',
			character == '_', character == '-':
			return character
		default:
			return '_'
		}
	}, slug)
	if replaced == "" {
		return "app"
	}
	return replaced
}
