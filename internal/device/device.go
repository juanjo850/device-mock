package device

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/protocol"
)

// Outcome is the result of handling a request.
type Outcome struct {
	Resp      protocol.Response
	Drop      bool // close connection without response
	Malformed bool // write garbage then close
}

// Device is a mock device instance.
type Device struct {
	ID         string
	Name       string
	ListenAddr string
	DelayMs    int
	Responses  map[string]any
	Behaviors  map[string]config.Behavior

	mu    sync.Mutex
	State map[string]any
}

// FromConfig builds a Device from config.
func FromConfig(c config.DeviceConfig) *Device {
	return &Device{
		ID:         c.ID,
		Name:       c.Name,
		ListenAddr: c.ListenAddr,
		DelayMs:    c.DelayMs,
		Responses:  c.Responses,
		Behaviors:  c.Behaviors,
		State:      map[string]any{},
	}
}

// Handle processes a request and returns an outcome.
func (d *Device) Handle(req protocol.Request) Outcome {
	// resolve behavior under lock
	d.mu.Lock()
	var b config.Behavior
	has := false
	if d.Behaviors != nil {
		if bb, ok := d.Behaviors[req.Method]; ok {
			b = bb
			has = true
		}
	}
	if !has && d.Responses != nil {
		if v, ok := d.Responses[req.Method]; ok {
			b = config.Behavior{Type: "static", Response: v, DelayMs: d.DelayMs}
			has = true
		}
	}
	if !has {
		b = config.Behavior{Type: "echo", DelayMs: d.DelayMs}
	}
	if b.DelayMs == 0 {
		b.DelayMs = d.DelayMs
	}
	d.mu.Unlock()

	if b.DelayMs > 0 {
		time.Sleep(time.Duration(b.DelayMs) * time.Millisecond)
	}

	// fault: timeout stalls
	if b.Type == "fault" && b.Fault == "timeout" {
		time.Sleep(30 * time.Second)
		return Outcome{Drop: true}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	switch b.Type {
	case "static":
		return Outcome{Resp: protocol.Response{ID: req.ID, Result: b.Response}}
	case "echo":
		var params any
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		return Outcome{Resp: protocol.Response{ID: req.ID, Result: map[string]any{"device": d.ID, "echo": params}}}
	case "state_get":
		key := b.StateKey
		if key == "" {
			key = "value"
		}
		return Outcome{Resp: protocol.Response{ID: req.ID, Result: d.State[key]}}
	case "state_set":
		var params map[string]any
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		key := b.StateKey
		if key == "" && params != nil {
			if k, ok := params["key"].(string); ok {
				key = k
			}
		}
		if key == "" {
			key = "value"
		}
		if params != nil {
			d.State[key] = params["value"]
		}
		return Outcome{Resp: protocol.Response{ID: req.ID, Result: map[string]any{"ok": true}}}
	case "fault":
		switch b.Fault {
		case "drop", "reset":
			return Outcome{Drop: true}
		case "malformed":
			return Outcome{Malformed: true}
		}
		return Outcome{Resp: protocol.Response{ID: req.ID, Error: &protocol.ErrorObj{Code: 500, Message: "unknown fault: " + b.Fault}}}
	default:
		var params any
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		return Outcome{Resp: protocol.Response{ID: req.ID, Result: map[string]any{"device": d.ID, "echo": params}}}
	}
}

// GetState returns a copy of device state.
func (d *Device) GetState() map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]any, len(d.State))
	for k, v := range d.State {
		out[k] = v
	}
	return out
}

// ResetState clears device state.
func (d *Device) ResetState() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.State = map[string]any{}
}

// Update applies new config fields safely.
func (d *Device) Update(cfg config.DeviceConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Name = cfg.Name
	d.DelayMs = cfg.DelayMs
	d.Responses = cfg.Responses
	d.Behaviors = cfg.Behaviors
}

// Registry holds configured devices.
type Registry struct {
	mu      sync.RWMutex
	devices map[string]*Device
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{devices: map[string]*Device{}}
}

// Add registers a device.
func (r *Registry) Add(d *Device) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.devices[d.ID] = d
}

// Get returns a device by id.
func (r *Registry) Get(id string) (*Device, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.devices[id]
	return d, ok
}

// Remove deletes a device.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.devices, id)
}

// List returns all devices.
func (r *Registry) List() []*Device {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Device, 0, len(r.devices))
	for _, d := range r.devices {
		out = append(out, d)
	}
	return out
}
