# device-mock

Mock device service: simulates one or more TCP devices speaking a generic
line/JSON protocol, for use by external clients.

## Hard rules
- No implementation code until the stack is agreed (see docs/spec.md § Stack).
- Do not commit secrets, credentials, or real device data.
- Keep this file short: hard rules + pointers only. Task detail lives in docs/spec.md / CONTEXT.md.
- Update CONTEXT.md after any meaningful change.

## Pointers
- Project spec / architecture (single source of truth): docs/spec.md
- Active context / repo-map (~10-20 lines): CONTEXT.md
- Stack: Go 1.27+ (see docs/spec.md §7).
