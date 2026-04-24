# Bannin Daemon

`daemon/` contains the local agent API, auditd log tailer, guided terminal setup, and backend dispatch logic used by Bannin.

## Backend URL Configuration

The daemon uses a hardcoded backend URL in `cmd/daemon/main.go`:

- `const backendURL = "http://localhost:3000"`

Update that constant if your backend is running elsewhere.

## Terminal Flow

`go run ./cmd/daemon init` now provides a guided auditd-only setup experience:

1. Install and configure auditd
2. Optionally collect a project path for backend context
3. Optionally request auditd rule generation from the hardcoded backend URL
4. Optionally restart auditd

## Tests

Run the full suite with a writable Go cache:

```bash
env GOCACHE=/tmp/go-cache go test ./...
```

The test suite covers both positive and negative paths for:

- install orchestration
- auditd installer behavior
- backend HTTP dispatching
- local receiver HTTP handlers
- auditd log ingestion
