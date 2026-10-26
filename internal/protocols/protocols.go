package protocols

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/juanjo850/device-mock/internal/device"
	"github.com/juanjo850/device-mock/internal/protocol"
)

// Handler serves a connection for a given protocol.
type Handler interface {
	Serve(ctx context.Context, conn net.Conn, d *device.Device, onRequest func())
}

// For returns the handler for a protocol name.
func For(proto string) Handler {
	switch proto {
	case "scpi":
		return SCPIHandler{}
	case "telnet":
		return TelnetHandler{}
	case "raw":
		return RawHandler{}
	default:
		return JSONHandler{}
	}
}

// --- JSON (original) ---

type JSONHandler struct{}

func (JSONHandler) Serve(ctx context.Context, conn net.Conn, d *device.Device, onRequest func()) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		if !scanner.Scan() {
			break
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		onRequest()
		req, err := protocol.DecodeRequest([]byte(line))
		if err != nil {
			resp := protocol.Response{Error: &protocol.ErrorObj{Code: 400, Message: err.Error()}}
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			writeJSONResp(conn, resp)
			continue
		}
		out := d.Handle(req)
		if out.Drop {
			return
		}
		if out.Malformed {
			_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, _ = conn.Write([]byte("{not valid json}\n"))
			return
		}
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		writeJSONResp(conn, out.Resp)
	}
	if err := scanner.Err(); err != nil {
		slog.Debug("read error", "device", d.ID, "err", err)
	}
}

func writeJSONResp(conn net.Conn, r protocol.Response) {
	b, err := protocol.Encode(r)
	if err != nil {
		return
	}
	b = append(b, '\n')
	_, _ = conn.Write(b)
}

// --- SCPI ---

type SCPIHandler struct{}

func (SCPIHandler) Serve(ctx context.Context, conn net.Conn, d *device.Device, onRequest func()) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		if !scanner.Scan() {
			break
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !handleTextCommand(conn, d, line, "\n", onRequest) {
			return
		}
	}
}

// --- Raw ---

type RawHandler struct{}

func (RawHandler) Serve(ctx context.Context, conn net.Conn, d *device.Device, onRequest func()) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		if !scanner.Scan() {
			break
		}
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !handleTextCommand(conn, d, line, "\n", onRequest) {
			return
		}
	}
}

// --- Telnet ---

type TelnetHandler struct{}

// IAC constants
const (
	iac     = 255
	will    = 251
	wont    = 252
	do      = 253
	dont    = 254
	sb      = 250
	se      = 240
	optEcho = 1
	optSGA  = 3
)

func (TelnetHandler) Serve(ctx context.Context, conn net.Conn, d *device.Device, onRequest func()) {
	defer conn.Close()
	// Announce capabilities.
	conn.Write([]byte{iac, will, optEcho})
	conn.Write([]byte{iac, will, optSGA})

	reader := bufio.NewReader(conn)
	var buf []byte
	textBuf := make([]byte, 0, 256)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		b, err := reader.ReadByte()
		if err != nil {
			if err != io.EOF {
				slog.Debug("telnet read error", "device", d.ID, "err", err)
			}
			return
		}

		if b == iac {
			// parse IAC sequence
			verb, err := reader.ReadByte()
			if err != nil {
				return
			}
			if verb == sb {
				// subnegotiation: read until IAC SE
				for {
					x, err := reader.ReadByte()
					if err != nil {
						return
					}
					if x == iac {
						next, err := reader.ReadByte()
						if err != nil {
							return
						}
						if next == se {
							break
						}
					}
				}
				continue
			}
			if verb == will || verb == wont || verb == do || verb == dont {
				opt, err := reader.ReadByte()
				if err != nil {
					return
				}
				// respond: agree to echo/sga, refuse others
				switch {
				case verb == do && (opt == optEcho || opt == optSGA):
					conn.Write([]byte{iac, will, opt})
				case verb == will && (opt == optEcho || opt == optSGA):
					conn.Write([]byte{iac, do, opt})
				case verb == do:
					conn.Write([]byte{iac, wont, opt})
				case verb == will:
					conn.Write([]byte{iac, dont, opt})
				}
				continue
			}
			// other 2-byte IAC (GA, etc.) already consumed verb; nothing more
			continue
		}

		if b == '\n' || b == '\r' {
			cmd := string(textBuf)
			textBuf = textBuf[:0]
			// consume paired \r\n
			// (best-effort: peek)
			if b == '\r' {
				// try read \n if available without blocking: use buffered
				// We'll just handle next byte next iteration; if it's \n skip via check
			}
			if strings.TrimSpace(cmd) == "" {
				continue
			}
			if !handleTextCommand(conn, d, cmd, "\r\n", onRequest) {
				return
			}
			continue
		}

		textBuf = append(textBuf, b)
		_ = buf
	}
}

// --- shared text command handling ---

func handleTextCommand(conn net.Conn, d *device.Device, cmd, terminator string, onRequest func()) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}
	onRequest()

	method := cmd
	if d.Protocol == "scpi" || d.Protocol == "telnet" {
		method = strings.ToUpper(method)
	}

	out := d.Handle(protocol.Request{Method: method})
	if out.Drop {
		return false
	}
	if out.Malformed {
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, _ = conn.Write([]byte("{not valid json}" + terminator))
		return false
	}
	text := formatOutcome(out.Resp)
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, _ = conn.Write([]byte(text + terminator))
	return true
}

func formatOutcome(r protocol.Response) string {
	if r.Error != nil {
		return r.Error.Message
	}
	if r.Result == nil {
		return ""
	}
	return fmt.Sprintf("%v", r.Result)
}
