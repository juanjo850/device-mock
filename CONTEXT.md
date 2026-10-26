# Active context

Goal: protocol adapters added (json, scpi, raw, telnet) on top of milestones 1-7.
Stack: Go 1.27+ (stdlib net, net/http ServeMux, slog, encoding/json).
Key files: cmd/device-mock/main.go, internal/{config,device,protocol,protocols,server,controlapi}.
Recent decisions:
- Per-device `protocol` field; handler interface in internal/protocols.
- SCPI case-insensitive command matching; telnet IAC negotiation.
- Control API exposes protocol; create supports protocol field.
Tests: go test ./... (all pass); gofmt/vet clean.
Next: tests for protocols package, docs, or feature work.
