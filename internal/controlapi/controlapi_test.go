package controlapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/device"
	"github.com/juanjo850/device-mock/internal/server"
	"github.com/stretchr/testify/require"
)

func newTestAPI(t *testing.T, token string) (*API, *server.Server) {
	t.Helper()
	reg := device.NewRegistry()
	d := device.FromConfig(config.DeviceConfig{ID: "d1", Name: "D1", ListenAddr: "127.0.0.1:19100", Responses: map[string]any{"ping": "pong"}})
	reg.Add(d)
	srv := server.New(reg)
	ctx := t.Context()
	srv.StartAll(ctx, reg.List())
	t.Cleanup(srv.Stop)
	return New(reg, srv, token, nil), srv
}

func TestHealthz(t *testing.T) {
	api, _ := newTestAPI(t, "")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Equal(t, "ok", body["status"])
}

func TestAuthRequired(t *testing.T) {
	api, _ := newTestAPI(t, "secret")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/devices", nil)
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/devices", nil)
	req.Header.Set("Authorization", "Bearer secret")
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateAndDeleteDevice(t *testing.T) {
	api, _ := newTestAPI(t, "")
	body := map[string]any{"id": "d2", "name": "D2", "listen_addr": "127.0.0.1:19101"}
	b, _ := json.Marshal(body)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/devices", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/devices/d2", nil)
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/devices/d2", nil)
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusNoContent, rr.Code)
}

func TestMetrics(t *testing.T) {
	api, _ := newTestAPI(t, "")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	api.Handler().ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), "device_mock_devices")
}
