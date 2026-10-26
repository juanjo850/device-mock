package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Behavior describes how a device handles a method.
type Behavior struct {
	Type     string `json:"type"` // static, echo, state_get, state_set, fault
	Response any    `json:"response,omitempty"`
	DelayMs  int    `json:"delay_ms,omitempty"`
	Fault    string `json:"fault,omitempty"` // drop, timeout, malformed, reset
	StateKey string `json:"state_key,omitempty"`
}

// DeviceConfig describes a single mock device.
type DeviceConfig struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	ListenAddr string              `json:"listen_addr"`
	Protocol   string              `json:"protocol,omitempty"` // json, scpi, telnet, raw
	DelayMs    int                 `json:"delay_ms"`
	Responses  map[string]any      `json:"responses,omitempty"`
	Behaviors  map[string]Behavior `json:"behaviors,omitempty"`
}

// Config is the top-level service configuration.
type Config struct {
	ControlAddr  string         `json:"control_addr"`
	ControlToken string         `json:"control_token,omitempty"`
	Devices      []DeviceConfig `json:"devices"`
}

// Load reads and parses a JSON config file.
func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Default returns a usable in-memory default config.
func Default() Config {
	return Config{
		ControlAddr: ":8080",
		Devices: []DeviceConfig{
			{
				ID:         "device-1",
				Name:       "Mock Device 1",
				Protocol:   "json",
				ListenAddr: ":9000",
				Responses: map[string]any{
					"ping":   "pong",
					"status": map[string]any{"ok": true},
				},
			},
		},
	}
}

// Validate checks config consistency.
func (c Config) Validate() error {
	if c.ControlAddr == "" {
		return errors.New("control_addr is required")
	}
	ids := map[string]bool{}
	addrs := map[string]bool{}
	for _, d := range c.Devices {
		if d.ID == "" {
			return errors.New("device id is required")
		}
		if ids[d.ID] {
			return fmt.Errorf("duplicate device id: %s", d.ID)
		}
		ids[d.ID] = true
		if d.ListenAddr == "" {
			return fmt.Errorf("device %s: listen_addr is required", d.ID)
		}
		if addrs[d.ListenAddr] {
			return fmt.Errorf("duplicate listen_addr: %s", d.ListenAddr)
		}
		addrs[d.ListenAddr] = true
		proto := normalizeProtocol(d.Protocol)
		if proto != "json" && proto != "scpi" && proto != "telnet" && proto != "raw" {
			return fmt.Errorf("device %s: unknown protocol: %s", d.ID, d.Protocol)
		}
		for method, beh := range d.Behaviors {
			if err := beh.Validate(); err != nil {
				return fmt.Errorf("device %s method %s: %w", d.ID, method, err)
			}
		}
	}
	return nil
}

// Validate checks a behavior.
func (b Behavior) Validate() error {
	switch b.Type {
	case "", "static", "echo", "state_get", "state_set":
	case "fault":
		switch b.Fault {
		case "drop", "timeout", "malformed", "reset":
		default:
			return fmt.Errorf("unknown fault: %s", b.Fault)
		}
	default:
		return fmt.Errorf("unknown behavior type: %s", b.Type)
	}
	return nil
}

func normalizeProtocol(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "json"
	}
	return p
}
