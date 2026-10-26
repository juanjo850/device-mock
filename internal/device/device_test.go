package device

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/protocol"
	"github.com/stretchr/testify/require"
)

func makeReq(t *testing.T, s string) protocol.Request {
	t.Helper()
	var r protocol.Request
	require.NoError(t, json.Unmarshal([]byte(s), &r))
	return r
}

func TestDeviceHandleConfiguredResponse(t *testing.T) {
	d := FromConfig(config.DeviceConfig{
		ID:         "d1",
		ListenAddr: ":9000",
		Responses:  map[string]any{"ping": "pong"},
	})
	out := d.Handle(makeReq(t, `{"id":"1","method":"ping"}`))
	require.False(t, out.Drop)
	require.Nil(t, out.Resp.Error)
	require.Equal(t, "pong", out.Resp.Result)
}

func TestDeviceHandleEchoDefault(t *testing.T) {
	d := FromConfig(config.DeviceConfig{ID: "d2", ListenAddr: ":9001"})
	out := d.Handle(makeReq(t, `{"id":"2","method":"unknown","params":{"a":1}}`))
	require.False(t, out.Drop)
	require.Nil(t, out.Resp.Error)
	m, ok := out.Resp.Result.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "d2", m["device"])
}

func TestDeviceDelay(t *testing.T) {
	d := FromConfig(config.DeviceConfig{ID: "d3", ListenAddr: ":9002", DelayMs: 50})
	start := time.Now()
	d.Handle(makeReq(t, `{"id":"3","method":"ping"}`))
	require.GreaterOrEqual(t, time.Since(start), 40*time.Millisecond)
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	d := FromConfig(config.DeviceConfig{ID: "d1", ListenAddr: ":9000"})
	r.Add(d)
	got, ok := r.Get("d1")
	require.True(t, ok)
	require.Equal(t, d, got)
	r.Remove("d1")
	_, ok = r.Get("d1")
	require.False(t, ok)
	require.Len(t, r.List(), 0)
}
