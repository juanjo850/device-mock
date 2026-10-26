# Active context

Goal: milestones 1-7 complete (unit tests, CI, Docker smoke, hot-reload).
Stack: Go 1.27+ (stdlib net, net/http ServeMux, slog, encoding/json).
Key files: cmd/device-mock/main.go, internal/{config,device,protocol,server,controlapi}.
Recent decisions:
- Optional bearer-token auth; /sessions, /metrics, enhanced /healthz.
- Hot-reload via SIGHUP and POST /reload (reconciles devices).
- CI workflow with Docker build + smoke test.
Tests: go test ./... (all pass); gofmt/vet clean.
Next: optional goreleaser release, in-band control, or feature work.
