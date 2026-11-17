# AGENTS.md

Go library (`github.com/eldius/cli-telemetry-helper`) that wires OpenTelemetry
(traces, metrics, logs over OTLP/gRPC) into cobra/viper CLI apps. There is no
executable in this repo — it is consumed by other projects.

## Verify changes

```sh
go build ./... && go vet ./...
```

- No tests exist (`*_test.go` count is 0); build + vet is the whole feedback loop.
- The `Makefile` targets (`search`, `sync`) are stale: they `go run ./cmd/cli`,
  but there is no `cmd/` directory. They will fail — don't use or "fix" them
  unless asked.

## Architecture

- `pkg/` is the public API; each package is a thin wrapper delegating to `internal/`.
  - `pkg/setup` — entrypoint: `InitViper` (config), `Setup` (tracer+meter+logger),
    `MainWrapper` (does both around a `main` func), `CobraCmdExecute` (runs a root
    cobra command inside a root span).
  - `pkg/telemetry` — span error helpers, counter constructor.
  - `pkg/http`, `pkg/db` — pre-instrumented `http.Client` (otelhttp) and
    `sqlx`/`sql` DBs (otelsql).
- `internal/telemetry` — OTLP/gRPC exporters for the three backends
  (all `WithInsecure()` + gzip), otelslog bridge, runtime metrics, and cobra
  `PersistentPreRunE`/`PersistentPostRunE` span hooks.
- Config shape (see `config.yaml`): `telemetry.backends.{traces,metrics,logs}`
  each with `endpoint`, `enabled`, `auth_headers`.

## Gotchas

- `Backend.Enabled` gates each backend, but only in `pkg/setup.Setup` —
  `internal/telemetry`'s `SetupTracer`/`SetupMeter`/`SetupLogger` don't check
  it. When the logs backend is disabled, `slog`'s default logger is never
  bridged to OTel (stays plain stderr).
- `InitViper` returns an error when no config file is found, so `MainWrapper`
  requires a config file on disk; defaults/env vars alone are not enough.
  Without `WithConfigFile`, lookup paths are `~/.<service-name>/` then `~/`.
- `${VAR}` expansion in config values happens only via the decode hook in
  `MainWrapper`'s `viper.UnmarshalKey("telemetry.backends", ...)` call — values
  read any other way are not expanded.
- `CobraCmdExecute` intentionally never calls `os.Exit` (it returns the code) so
  the caller can run the `Setup` shutdown func first; skipping the shutdown func
  loses buffered spans/metrics (batch exporters).
- `getDefaultResources` (internal/telemetry/span.go) caches the resource in a
  package global — the service name/version from the first `Setup` call wins for
  the process lifetime.
- Instrumentation scope names are hardcoded (`"cli-telemetry-poc"`, `"main"`),
  including a hardcoded `service.name="cli-telemetry-poc"` span attribute in
  `CobraCmdExecute` that does not follow the configured service name.
