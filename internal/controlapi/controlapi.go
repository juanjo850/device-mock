package controlapi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/device"
	"github.com/juanjo850/device-mock/internal/server"
)

// API exposes HTTP control endpoints.
type API struct {
	registry *device.Registry
	server   *server.Server
	token    string
	reload   func() error
	mux      *http.ServeMux
}

// New creates a control API.
func New(registry *device.Registry, srv *server.Server, token string, reload func() error) *API {
	a := &API{registry: registry, server: srv, token: token, reload: reload, mux: http.NewServeMux()}
	a.routes()
	return a
}

// Handler returns the http.Handler (with auth middleware).
func (a *API) Handler() http.Handler {
	return a.authMiddleware(a.mux)
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", a.healthz)
	a.mux.HandleFunc("GET /metrics", a.metrics)
	a.mux.HandleFunc("GET /sessions", a.listSessions)
	a.mux.HandleFunc("GET /devices", a.listDevices)
	a.mux.HandleFunc("GET /devices/{id}", a.getDevice)
	a.mux.HandleFunc("GET /devices/{id}/state", a.getState)
	a.mux.HandleFunc("DELETE /devices/{id}/state", a.resetState)
	a.mux.HandleFunc("POST /devices", a.createDevice)
	a.mux.HandleFunc("DELETE /devices/{id}", a.deleteDevice)
	a.mux.HandleFunc("POST /reload", a.reloadConfig)
}

func (a *API) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+a.token {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *API) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"devices":         len(a.registry.List()),
		"active_sessions": a.server.ActiveSessions(),
	})
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	sessions := a.server.ListSessions()
	out := make([]map[string]any, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, map[string]any{
			"id":           s.ID,
			"device_id":    s.DeviceID,
			"remote_addr":  s.RemoteAddr,
			"connected_at": s.ConnectedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "device_mock_devices %d\n", len(a.registry.List()))
	fmt.Fprintf(w, "device_mock_active_sessions %d\n", a.server.ActiveSessions())
	fmt.Fprintf(w, "device_mock_connections_total %d\n", a.server.ConnectionsTotal())
	fmt.Fprintf(w, "device_mock_requests_total %d\n", a.server.RequestsTotal())
}

func (a *API) listDevices(w http.ResponseWriter, r *http.Request) {
	devs := a.registry.List()
	out := make([]map[string]any, 0, len(devs))
	for _, d := range devs {
		out = append(out, map[string]any{
			"id":          d.ID,
			"name":        d.Name,
			"protocol":    d.Protocol,
			"listen_addr": d.ListenAddr,
			"delay_ms":    d.DelayMs,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, ok := a.registry.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          d.ID,
		"name":        d.Name,
		"protocol":    d.Protocol,
		"listen_addr": d.ListenAddr,
		"delay_ms":    d.DelayMs,
		"responses":   d.Responses,
		"behaviors":   d.Behaviors,
		"state":       d.GetState(),
	})
}

func (a *API) getState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, ok := a.registry.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, d.GetState())
}

func (a *API) resetState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, ok := a.registry.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		return
	}
	d.ResetState()
	w.WriteHeader(http.StatusNoContent)
}

type createReq struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Protocol   string                     `json:"protocol"`
	ListenAddr string                     `json:"listen_addr"`
	DelayMs    int                        `json:"delay_ms"`
	Responses  map[string]any             `json:"responses"`
	Behaviors  map[string]config.Behavior `json:"behaviors"`
}

func (a *API) createDevice(w http.ResponseWriter, r *http.Request) {
	var cr createReq
	if err := json.NewDecoder(r.Body).Decode(&cr); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if cr.ID == "" || cr.ListenAddr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id and listen_addr required"})
		return
	}
	d := device.FromConfig(config.DeviceConfig{
		ID:         cr.ID,
		Name:       cr.Name,
		Protocol:   cr.Protocol,
		ListenAddr: cr.ListenAddr,
		DelayMs:    cr.DelayMs,
		Responses:  cr.Responses,
		Behaviors:  cr.Behaviors,
	})
	if err := a.server.AddDevice(r.Context(), d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": d.ID})
}

func (a *API) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.server.RemoveDevice(id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) reloadConfig(w http.ResponseWriter, r *http.Request) {
	if a.reload == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "reload not configured"})
		return
	}
	if err := a.reload(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reloaded"})
}
