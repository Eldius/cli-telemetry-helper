# AGENTS.md

Go library (`github.com/eldius/cli-telemetry-helper`) that wires OpenTelemetry
(traces, metrics, logs over OTLP/gRPC) into cobra/viper CLI apps. There is no
executable in this repo — it is consumed by other projects.

## Verify changes

```sh
go build ./... && go vet ./...
```

- No tests exist (`*_test.go` count is 0); build + vet is the core feedback loop.
- `make lint` (golangci-lint, no config file in repo → default linters) also passes.
- `make vulncheck` runs `go tool govulncheck` (pinned via go.mod's `tool` directive).
- `make release` runs `git tag` + `git push --tags` with an auto-bumped patch
  version — git-mutating, don't run unless explicitly asked.

## Architecture

- `pkg/` is the public API; each package is a thin wrapper delegating to `internal/`.
  - `pkg/setup` — entrypoint: `InitViper` (config), `Setup` (tracer+meter+logger),
    `MainWrapper` (does both around a `main` func), `CobraCmdExecute` (runs a root
    cobra command inside a root span), and config getters (`Get`, `GetString`,
    `GetBool`, `GetInt`, `GetInt64`, `GetFloat64`, `GetDuration`, `GetStringSlice`,
    `GetStringMapString`, `UnmarshalKey`) that expand `${VAR}` env references —
    the only read path that does.
  - `pkg/telemetry` — span error helpers, counter constructor.
  - `pkg/http`, `pkg/db` — pre-instrumented `http.Client` (otelhttp) and
    `sqlx`/`sql` DBs (otelsql).
- `internal/telemetry` — OTLP/gRPC exporters for the three signals
  (all `WithInsecure()` + gzip), otelslog bridge, runtime metrics, and cobra
  `PersistentPreRunE`/`PersistentPostRunE` span hooks.
- Config shape (see `config.yaml`): `telemetry.backends.{traces,metrics,logs}`
  each with `endpoint`, `enabled`, `auth_headers`.

## Gotchas

- `Backend.Enabled` gates each signal, but only in `pkg/setup.Setup` —
  `internal/telemetry`'s `SetupTracer`/`SetupMeter`/`SetupLogger` don't check
  it. When the logs backend is disabled, `slog`'s default logger is never
  bridged to OTel (stays plain stderr).
- `InitViper` swallows *any* `ReadInConfig` error — missing *or malformed*
  config file only logs "using default configs" at Info and returns nil, so a
  broken config silently falls back to defaults/env vars. Without
  `WithConfigFile`, lookup order is `WithConfigFileLookupDirectories` dirs,
  then `~/.<service-name>/`, then `~/`.
- Similarly, `MainWrapper` only warns (not fails) when
  `viper.UnmarshalKey("telemetry.backends", ...)` fails — telemetry then runs
  with zero-value backends (all disabled).
- `${VAR}` expansion in config values happens only through the `pkg/setup`
  wrappers (`Get`, `GetString`, `GetInt`, …, `UnmarshalKey`) — plain
  `viper.Get*` / `viper.UnmarshalKey` calls do not expand.
- `CobraCmdExecute` never calls `os.Exit` (it returns an error) so the caller
  can run the shutdown func from `Setup` first; skipping it loses buffered
  spans/metrics/logs (batch exporters). `MainWrapper` defers shutdown itself.
- `getDefaultResources` (internal/telemetry/span.go) caches the resource in a
  package global — the service name/version from the first `Setup` call wins for
  the process lifetime.
- All instrumentation scopes share one name, `telemetry.InstrumentationName`
  (the module path, defined in internal/telemetry/telemetry.go); `service.name`
  comes from the configured resource, not from span attributes.
- `pkg/http.NewHTTPClient` additionally reads and logs full request/response
  bodies at debug level (4 KiB cap; Authorization/Cookie/X-Api-Key etc.
  redacted) via its own RoundTripper wrapper around otelhttp.
