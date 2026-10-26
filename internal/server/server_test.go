package server

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/device"
	"github.com/stretchr/testify/require"
)

func TestServerRoundTrip(t *testing.T) {
	reg := device.NewRegistry()
	d := device.FromConfig(config.DeviceConfig{
		ID:         "d1",
		ListenAddr: "127.0.0.1:19000",
		Responses:  map[string]any{"ping": "pong"},
	})
	reg.Add(d)
	srv := New(reg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.StartAll(ctx, reg.List())
	defer srv.Stop()

	// give listener a moment
	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", "127.0.0.1:19000")
	require.NoError(t, err)
	defer conn.Close()
	_, _ = conn.Write([]byte(`{"id":"1","method":"ping"}` + "\n"))
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"1","result":"pong"}`, line)
}

func TestServerSCPI(t *testing.T) {
	reg := device.NewRegistry()
	d := device.FromConfig(config.DeviceConfig{
		ID:         "scpi",
		Protocol:   "scpi",
		ListenAddr: "127.0.0.1:19002",
		Responses:  map[string]any{"*IDN?": "ACME,1.0"},
	})
	reg.Add(d)
	srv := New(reg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.StartAll(ctx, reg.List())
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", "127.0.0.1:19002")
	require.NoError(t, err)
	defer conn.Close()
	_, _ = conn.Write([]byte("*idn?\n"))
	r := bufio.NewReader(conn)
	line, err := r.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "ACME,1.0\n", line)
}
