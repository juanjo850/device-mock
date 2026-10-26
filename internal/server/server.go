package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/juanjo850/device-mock/internal/config"
	"github.com/juanjo850/device-mock/internal/device"
	"github.com/juanjo850/device-mock/internal/protocols"
)

// Session describes an active client connection.
type Session struct {
	ID          string
	DeviceID    string
	RemoteAddr  string
	ConnectedAt time.Time
}

// Server manages TCP listeners for devices.
type Server struct {
	registry *device.Registry

	mu        sync.Mutex
	listeners map[string]net.Listener
	cancelFns map[string]context.CancelFunc
	ctx       context.Context

	sessionsMu sync.Mutex
	sessions   map[string]*Session

	connTotal atomic.Int64
	reqTotal  atomic.Int64
	nextID    atomic.Int64

	wg sync.WaitGroup
}

// New creates a Server.
func New(registry *device.Registry) *Server {
	return &Server{
		registry:  registry,
		listeners: map[string]net.Listener{},
		cancelFns: map[string]context.CancelFunc{},
		sessions:  map[string]*Session{},
	}
}

// StartDevice starts a TCP listener for a device.
func (s *Server) StartDevice(ctx context.Context, d *device.Device) error {
	ln, err := net.Listen("tcp", d.ListenAddr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listeners[d.ID] = ln
	ctx2, cancel := context.WithCancel(ctx)
	s.cancelFns[d.ID] = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-ctx2.Done():
					return
				default:
				}
				return
			}
			s.wg.Add(1)
			go func(c net.Conn) {
				defer s.wg.Done()
				s.handleConn(ctx2, d, c)
			}(conn)
		}
	}()
	return nil
}

// StartAll starts listeners for all given devices.
func (s *Server) StartAll(ctx context.Context, devs []*device.Device) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	for _, d := range devs {
		if err := s.StartDevice(ctx, d); err != nil {
			slog.Error("failed to start device listener", "device", d.ID, "addr", d.ListenAddr, "err", err)
		} else {
			slog.Info("device listening", "device", d.ID, "addr", d.ListenAddr)
		}
	}
}

// Stop closes all listeners.
func (s *Server) Stop() {
	s.mu.Lock()
	for _, cancel := range s.cancelFns {
		cancel()
	}
	for _, ln := range s.listeners {
		_ = ln.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

const (
	idleTimeout  = 5 * time.Minute
	writeTimeout = 10 * time.Second
)

// Reload reconciles running devices with a new config.
func (s *Server) Reload(ctx context.Context, cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	newByID := map[string]config.DeviceConfig{}
	for _, dc := range cfg.Devices {
		newByID[dc.ID] = dc
	}

	// remove devices not present in new config
	for _, d := range s.registry.List() {
		if _, ok := newByID[d.ID]; !ok {
			slog.Info("reload: removing device", "device", d.ID)
			s.RemoveDevice(d.ID)
		}
	}

	// add or update
	for _, dc := range cfg.Devices {
		if existing, ok := s.registry.Get(dc.ID); ok {
			if existing.ListenAddr != dc.ListenAddr {
				slog.Info("reload: recreating device (addr changed)", "device", dc.ID)
				s.RemoveDevice(dc.ID)
				nd := device.FromConfig(dc)
				s.registry.Add(nd)
				if err := s.StartDevice(ctx, nd); err != nil {
					return err
				}
			} else {
				slog.Info("reload: updating device", "device", dc.ID)
				existing.Update(dc)
			}
		} else {
			slog.Info("reload: adding device", "device", dc.ID)
			nd := device.FromConfig(dc)
			s.registry.Add(nd)
			if err := s.StartDevice(ctx, nd); err != nil {
				return err
			}
		}
	}
	return nil
}

// ListSessions returns a snapshot of active sessions.
func (s *Server) ListSessions() []Session {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	out := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		out = append(out, *sess)
	}
	return out
}

// ActiveSessions returns the number of active connections.
func (s *Server) ActiveSessions() int {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	return len(s.sessions)
}

// ConnectionsTotal returns total connections accepted.
func (s *Server) ConnectionsTotal() int64 { return s.connTotal.Load() }

// RequestsTotal returns total requests handled.
func (s *Server) RequestsTotal() int64 { return s.reqTotal.Load() }

// handleConn serves a single client connection for a device.
func (s *Server) handleConn(ctx context.Context, d *device.Device, conn net.Conn) {
	sid := fmt.Sprintf("s-%d", s.nextID.Add(1))
	sess := &Session{ID: sid, DeviceID: d.ID, RemoteAddr: conn.RemoteAddr().String(), ConnectedAt: time.Now()}
	s.sessionsMu.Lock()
	s.sessions[sid] = sess
	s.sessionsMu.Unlock()
	defer func() {
		s.sessionsMu.Lock()
		delete(s.sessions, sid)
		s.sessionsMu.Unlock()
	}()
	s.connTotal.Add(1)
	slog.Info("client connected", "device", d.ID, "protocol", d.Protocol, "session", sid, "remote", conn.RemoteAddr().String())
	defer slog.Info("client disconnected", "device", d.ID, "session", sid, "remote", conn.RemoteAddr().String())

	h := protocols.For(d.Protocol)
	h.Serve(ctx, conn, d, func() { s.reqTotal.Add(1) })
}

// AddDevice registers and starts a device at runtime.
func (s *Server) AddDevice(ctx context.Context, d *device.Device) error {
	s.registry.Add(d)
	s.mu.Lock()
	base := s.ctx
	s.mu.Unlock()
	if base == nil {
		base = context.Background()
	}
	return s.StartDevice(base, d)
}

// RemoveDevice stops and removes a device.
func (s *Server) RemoveDevice(id string) {
	s.mu.Lock()
	if cancel, ok := s.cancelFns[id]; ok {
		cancel()
	}
	if ln, ok := s.listeners[id]; ok {
		_ = ln.Close()
		delete(s.listeners, id)
		delete(s.cancelFns, id)
	}
	s.mu.Unlock()
	s.registry.Remove(id)
}
