# device-mock

A small, self-contained **mock device service** for development and testing.
It simulates one or more TCP devices that speak a generic **newline-delimited
JSON** protocol, and exposes an **HTTP control API** to manage them at runtime.

Built in Go. Single binary, Docker-friendly, no external dependencies at
runtime.

---

## Table of contents

- [Features](#features)
- [Quick start](#quick-start)
- [Protocol](#protocol)
- [Configuration](#configuration)
- [Behaviors](#behaviors)
- [Control API](#control-api)
- [Authentication](#authentication)
- [Hot reload](#hot-reload)
- [Observability](#observability)
- [Examples](#examples)
- [Build & test](#build--test)
- [Docker](#docker)
- [Project layout](#project-layout)

---

## Features

- **Multiple devices**, each on its own TCP port.
- **Request/response** over newline-delimited JSON (any method name).
- **Static responses** and a default **echo**.
- **Stateful behaviors**: get/set device state across requests.
- **Fault injection**: drop connection, stall (timeout), malformed response, reset.
- **Per-method and per-device delays**.
- **Runtime management** via HTTP control API: add/remove devices, reset state.
- **Session tracking** + Prometheus-style metrics.
- **Optional bearer-token auth** on the control API.
- **Hot reload** of config via `SIGHUP` or `POST /reload`.
- **Graceful shutdown**.

---

## Quick start

```sh
# Build
make build

# Run with the bundled config
make run
# or
./bin/device-mock -config configs/devices.json
```

By default:
- Device 1 listens on `:9000`
- Device 2 listens on `:9001`
- Control API listens on `:8080`

Try it:

```sh
printf '{"id":"1","method":"ping"}\n' | nc localhost 9000
# {"id":"1","result":"pong"}
```

---

## Protocols

Each device declares a `protocol` (default `json`):

| Protocol | Description |
|----------|-------------|
| `json`   | Newline-delimited JSON request/response (default). |
| `scpi`   | SCPI-style text commands (case-insensitive), text responses. |
| `raw`    | Line-delimited text, exact-match responses. |
| `telnet` | Telnet with IAC negotiation, then text commands. |

### JSON (default)

Transport is **TCP**. Each message is one JSON object terminated by a newline
(`\n`). Requests and responses are matched by the optional `id` field.

### Request

```json
{"id":"1","method":"ping"}
{"id":"2","method":"set","params":{"key":"counter","value":5}}
```

| Field    | Type            | Required | Description                       |
|----------|-----------------|----------|-----------------------------------|
| `id`     | string/number   | no       | echoed back in the response       |
| `method` | string          | yes      | method name                       |
| `params` | any             | no       | arbitrary JSON payload            |

### Response

```json
{"id":"1","result":"pong"}
{"id":"2","result":{"ok":true}}
{"id":"3","error":{"code":400,"message":"unknown method"}}
```

A connection stays open after a response (persistent). Send as many requests
as you like; close the socket to end the session. Idle connections time out
after 5 minutes.

---

## Configuration

Config is a single JSON file:

```json
{
  "control_addr": ":8080",
  "control_token": "",
  "devices": [
    {
      "id": "device-1",
      "name": "Mock Device 1",
      "listen_addr": ":9000",
      "delay_ms": 0,
      "responses": {
        "ping": "pong",
        "status": { "ok": true }
      },
      "behaviors": {
        "get_counter": { "type": "state_get", "state_key": "counter" },
        "set_counter": { "type": "state_set", "state_key": "counter" },
        "boom":        { "type": "fault", "fault": "malformed" }
      }
    }
  ]
}
```

| Field           | Description                                              |
|-----------------|----------------------------------------------------------|
| `control_addr`  | HTTP control API listen address                          |
| `control_token` | Optional bearer token (empty = no auth)                  |
| `devices[].id`  | Unique device id                                         |
| `devices[].name`| Human-readable name                                      |
| `devices[].listen_addr` | TCP listen address for this device                   |
| `devices[].delay_ms`    | Default delay applied to every method (ms)           |
| `devices[].responses`   | Map of method → static response                      |
| `devices[].behaviors`   | Map of method → behavior (overrides `responses`)     |

Device ids and listen addresses must be unique. Validation runs on load.

---

## Behaviors

A behavior decides how a method is handled. If a method has a behavior it
takes precedence over `responses`; otherwise the method falls back to its
static response, or to **echo** (returns `{"device":..., "echo": params}`).

| `type`      | Description                                                          |
|-------------|----------------------------------------------------------------------|
| `static`    | Return `response` as-is.                                             |
| `echo`      | Echo back params (default for unknown methods).                       |
| `state_get` | Return the value stored under `state_key` (default `"value"`).        |
| `state_set` | Store `params.value` under `state_key` (or `params.key`). Returns `{"ok":true}`. |
| `fault`     | Inject a fault (see below).                                          |

Each behavior may also set `delay_ms` to stall before responding.

### Faults

| `fault`     | Effect                                                         |
|-------------|----------------------------------------------------------------|
| `drop`      | Close the connection without a response.                        |
| `timeout`   | Stall for 30s then drop (simulates a hung device).             |
| `malformed` | Write `{not valid json}` and close.                            |
| `reset`     | Close the connection abruptly.                                  |

### SCPI example

```sh
printf '*IDN?\n' | nc localhost 9100
# ACME,MODEL1,12345,1.0
printf 'MEAS:VOLT?\n' | nc localhost 9100
# 3.14
```

Config:

```json
{
  "id": "scpi-dev",
  "protocol": "scpi",
  "listen_addr": ":9100",
  "responses": {
    "*IDN?": "ACME,MODEL1,12345,1.0",
    "MEAS:VOLT?": 3.14
  }
}
```

### Raw example

```sh
printf 'PING\n' | nc localhost 9101
# PONG
```

### Telnet example

```sh
telnet localhost 9102
# then type: HELLO  ->  HI
```

### Stateful example

```
→ {"id":"1","method":"set_counter","params":{"value":42}}
← {"id":"1","result":{"ok":true}}

→ {"id":"2","method":"get_counter"}
← {"id":"2","result":42}
```

State persists across requests on the same or different connections until
reset via the control API.

---

## Control API

Base URL: `http://<host>:<control_port>` (default `:8080`).

| Method   | Path                      | Description                              |
|----------|---------------------------|------------------------------------------|
| GET      | `/healthz`                | Liveness + device/session counts         |
| GET      | `/metrics`                | Prometheus text metrics                   |
| GET      | `/sessions`               | Active connections                       |
| GET      | `/devices`                | List devices                             |
| GET      | `/devices/{id}`           | Device details, behaviors, state         |
| POST     | `/devices`                | Create a device                          |
| DELETE   | `/devices/{id}`           | Remove a device                          |
| GET      | `/devices/{id}/state`     | Device state                             |
| DELETE   | `/devices/{id}/state`     | Reset device state                       |
| POST     | `/reload`                 | Reload config from disk                  |

### Create device

```sh
curl -X POST localhost:8080/devices \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "sensor-1",
    "name": "Sensor 1",
    "listen_addr": ":9002",
    "responses": { "read": 23.5 }
  }'
```

### State

```sh
curl localhost:8080/devices/device-1/state
curl -X DELETE localhost:8080/devices/device-1/state
```

---

## Authentication

Set `control_token` in the config (or pass via config file). When set, every
control endpoint except `/healthz` requires:

```
Authorization: Bearer <token>
```

Example:

```sh
curl -H 'Authorization: Bearer secret123' localhost:8080/devices
```

Leave `control_token` empty to disable auth (dev mode).

---

## Hot reload

Reload config without restarting:

```sh
# via signal
kill -HUP <pid>

# or via API
curl -X POST localhost:8080/reload
```

Devices present in the new config are added or updated in place; devices
removed from the config are stopped and removed. Changing a device's
`listen_addr` recreates its listener.

---

## Observability

- **Structured logs** via `slog`: connection open/close, listener start/fail,
  reload events. Set `LOG_LEVEL`? (default info).
- **`/healthz`** returns `{status, devices, active_sessions}`.
- **`/metrics`** (Prometheus text):
  - `device_mock_devices`
  - `device_mock_active_sessions`
  - `device_mock_connections_total`
  - `device_mock_requests_total`
- **`/sessions`** lists active connections with device, remote address, and
  connect time.

---

## Examples

### Python client

```python
import socket, json

s = socket.create_connection(("localhost", 9000))
s.sendall(b'{"id":1,"method":"ping"}\n')
print(s.recv(1024).decode().strip())
s.close()
```

### Simulate a flaky device

```json
{
  "id": "flaky",
  "listen_addr": ":9003",
  "behaviors": {
    "read": { "type": "fault", "fault": "drop" }
  }
}
```

### Simulate a slow device

```json
{
  "id": "slow",
  "listen_addr": ":9004",
  "delay_ms": 2000,
  "responses": { "ping": "pong" }
}
```

---

## Build & test

```sh
make build   # builds bin/device-mock
make run     # runs with configs/devices.json
make test    # go test ./...
make tidy    # go mod tidy
make docker  # builds Docker image
```

Requirements: Go 1.27+.

---

## Docker

```sh
make docker
docker run -p 8080:8080 -p 9000:9000 -p 9001:9001 device-mock:latest
```

The image uses a distroless base and bundles `configs/devices.json`.

---

## Project layout

```
device-mock/
├── cmd/device-mock/        # entrypoint
├── internal/
│   ├── config/             # config loading + validation
│   ├── device/             # Device + Registry
│   ├── protocol/           # line/JSON framing
│   ├── server/             # TCP listeners, sessions
│   └── controlapi/         # HTTP control API
├── configs/devices.json    # example config
├── docs/spec.md            # project spec (source of truth)
├── AGENTS.md               # agent memory (rules + pointers)
├── CONTEXT.md              # active context / repo-map
├── Dockerfile
├── Makefile
└── .github/workflows/ci.yml
```

---

## Status

Milestones 1–7 complete: context/spec, stack (Go), minimal TCP mock,
multi-device + registry + config, behavior engine + hardening, control API +
observability (auth, sessions, metrics, hot reload), tests + CI + Docker.

Optional future work: goreleaser releases, in-band control commands, scenario
scripts, hot-reload of TLS, persistence of runtime devices.
