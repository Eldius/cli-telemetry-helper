# cli-telemetry-helper

Go library that wires OpenTelemetry (traces, metrics, and logs over OTLP/gRPC)
into cobra/viper CLI applications. It gives a CLI a root span per execution,
per-command spans, Go runtime metrics, and an `slog`-to-OTel log bridge, plus
pre-instrumented HTTP and database clients.

## Install

```sh
go get github.com/eldius/cli-telemetry-helper
```

## Quick start

Wire your `main` through `MainWrapper` (config + telemetry setup, flush on
return), run your root command with `CobraCmdExecute` (root span, never calls
`os.Exit`), and attach the span hooks to your root command:

```go
package main

import (
	"context"
	"os"

	"github.com/eldius/cli-telemetry-helper/pkg/setup"
	"github.com/spf13/cobra"
)

func main() {
	svc := setup.Service{Name: "my-cli", Version: "0.1.0"}

	rootCmd := &cobra.Command{
		Use:                "my-cli",
		PersistentPreRunE:  setup.PersistentPreRunE,  // starts a span per command
		PersistentPostRunE: setup.PersistentPostRunE, // ends it
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Context() // carries the command span
			return nil
		},
	}

	ctx := context.Background()
	err := setup.MainWrapper(ctx, svc, func(ctx context.Context) error {
		return setup.CobraCmdExecute(ctx, rootCmd)
	},
		setup.WithConfigFileName("config"),
		setup.WithConfigFileType("yaml"),
	)
	if err != nil {
		os.Exit(1)
	}
}
```

`InitViper` options: `WithConfigFile` (explicit path), `WithConfigFileName` /
`WithConfigFileType`, `WithConfigFileLookupDirectories`,
`WithDefaultConfigs`, `WithEnvPrefix`. Without `WithConfigFile`, the config
file is searched in the lookup directories, then `~/.<service-name>/`, then
`~/`; environment variables override config values (`.` in keys maps to `_`).

## Configuration

Backends are configured under `telemetry.backends` (see [`config.yaml`](config.yaml)):

```yaml
telemetry:
  backends:
    traces:
      endpoint: localhost:4317
      enabled: true
      auth_headers:
        authorization: "${OTLP_AUTH_TOKEN}"
    metrics:
      endpoint: localhost:4317
      enabled: true
    logs:
      endpoint: localhost:4317
      enabled: false
```

Each signal (`traces`, `metrics`, `logs`) is exported independently over
OTLP/gRPC (insecure, gzip). Disabled signals are skipped entirely — if `logs`
is disabled, `slog` keeps logging to stderr instead of OTel.

## Reading config values (`${VAR}` expansion)

Only the getters in `pkg/setup` expand `${VAR}` environment references in
config values — plain `viper.Get*` calls do not:

```go
setup.GetString("server.host")        // "${HOST}:8080" -> "example.com:8080"
setup.GetInt("server.port")           // "${PORT}" -> 9090
setup.GetDuration("server.timeout")   // "${TIMEOUT}" -> 5s
setup.GetStringSlice("server.tags")   // expands each element
setup.GetStringMapString("labels")    // expands each value

var cfg MyConfig
err := setup.UnmarshalKey("server", &cfg) // expands while decoding structs
```

Also available: `Get`, `GetBool`, `GetInt64`, `GetFloat64`.

## Instrumented clients

```go
// HTTP client: otelhttp tracing + debug-level request/response logging
// (bodies capped at 4 KiB, auth/cookie headers redacted).
client := http.NewHTTPClient()

// database/sql or sqlx connections instrumented with otelsql.
sqlDB, err := db.GetDB("postgres", connStr)
sqlxDB, err := db.GetSqlxDB("postgres", connStr)
```

(`pkg/http` and `pkg/db` import paths are
`github.com/eldius/cli-telemetry-helper/pkg/http` and `.../pkg/db`.)

## Span and metric helpers

```go
import "github.com/eldius/cli-telemetry-helper/pkg/telemetry"

telemetry.RecordError(ctx, err)              // record on the current span
return telemetry.RecordErrorAndClose(ctx, err) // record + end span

counter := telemetry.NewInt64CounterMeter("my_counter")
counter.Add(ctx, 1)
```

## Exit codes and flushing

`CobraCmdExecute` returns an error instead of calling `os.Exit` so buffered
telemetry can be flushed first. If you call `Setup` directly instead of
`MainWrapper`, always invoke the returned shutdown function before exiting or
buffered spans/metrics/logs are lost:

```go
shutdown, err := setup.Setup(ctx, cfg)
if err != nil { /* ... */ }
defer shutdown()
```

## License

See [LICENSE](LICENSE).
