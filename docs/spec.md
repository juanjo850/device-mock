# Project Spec — device-mock

> Single source of truth for "what this project is and why."
> Keep task-specific detail here; keep AGENTS.md to rules + pointers.

## 1. Purpose
Mock the connection and communication to one or more devices so external clients
can develop and test against a controllable device endpoint without real hardware.

## 2. Scope
### In scope
- Simulate multiple concurrent device endpoints over TCP.
- Generic line/JSON protocol: newline-delimited messages, JSON payloads.
- Configurable device behavior (responses, delays, errors, state).
- Observable connection lifecycle (connect, disconnect, timeouts).
- Management/control surface to configure mock devices at runtime.

### Out of scope (for now)
- Real hardware integration.
- Protocol-specific stacks (Modbus, MQTT, OPC-UA) — may be added later.
- Production-grade security/auth (mock only).

## 3. Consumers
External clients connect over TCP. The service must be runnable standalone and
addressable by host:port.

## 4. Protocol (initial)
- Transport: TCP.
- Framing: newline-delimited (one JSON object per line) OR raw lines (configurable).
- Encoding: UTF-8.
- Direction: request/response + optional server-pushed events.
- Errors: JSON error objects with code + message.

## 5. Architecture (high level)
Components (names provisional until stack chosen):
- **Listener/Server**: accepts TCP connections, manages sockets.
- **Device Registry**: holds configured mock devices and their state.
- **Protocol Handler**: parses/serializes line/JSON messages.
- **Behavior Engine**: decides responses, delays, faults per device profile.
- **Control API**: runtime configuration (add/remove devices, set behaviors).
- **Logger/Metrics**: connection and message observability.

## 6. Non-functional requirements
- Concurrency: handle many simultaneous client connections.
- Configurability: device profiles via file (JSON/YAML) and runtime API.
- Determinism: reproducible scenarios for tests.
- Observability: structured logs for connect/disconnect/message events.
- Portability: runnable on developer machines; container-friendly.

## 7. Stack (decided)
- Language: Go 1.27+ (modules).
- TCP devices: stdlib `net` package, one goroutine per connection.
- Concurrency: goroutines + channels + `sync`.
- Control API: stdlib `net/http` with ServeMux (Go 1.22+ patterns), separate port.
- Config: JSON via `encoding/json` (device profiles as JSON files).
- Logging: stdlib `log/slog`.
- Testing: `testing` + `github.com/stretchr/testify`.
- Packaging: go modules, Dockerfile, Makefile.

## 7.1 Control API
HTTP REST on a separate port. Endpoints TBD during scaffolding (see milestones).

## 8. Open questions
- Single process vs. multi-process device simulation?
- Control API transport: decided = HTTP REST (separate port).
- Persistence of scenarios across restarts?
- Auth on control API?

## 9. Milestones
1. Context + spec (this doc). ✅
2. Stack decision (Go). ✅
3. Minimal TCP mock device (one device, request/response). ✅
4. Multi-device + registry + config file. ✅
5. Behavior engine (delays, errors, state) + hardening (deadlines, validation, logging). ✅
6. Control API + observability (auth, sessions, metrics). ✅
7. Tests + packaging — done (unit/integration tests, CI workflow, Docker build + smoke test). Remaining optional: goreleaser release, in-band control.
