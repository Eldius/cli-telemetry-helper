package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// resetViper gives each test a clean global viper instance; the functions
// under test operate on the viper singleton.
func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

// writeConfig writes content to a config.yaml in a temp dir and returns its path.
func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestInitViperEmptyServiceName(t *testing.T) {
	resetViper(t)
	err := InitViper(context.Background(), Service{})
	if !errors.Is(err, ErrEmptyAppName) {
		t.Fatalf("err = %v, want ErrEmptyAppName", err)
	}
}

func TestInitViperMissingConfigFileIsTolerated(t *testing.T) {
	resetViper(t)
	err := InitViper(context.Background(), Service{Name: "app"},
		WithConfigFileLookupDirectories(t.TempDir()),
		WithConfigFileName("missing"),
		WithConfigFileType("yaml"),
	)
	if err != nil {
		t.Fatalf("err = %v, want nil (missing config falls back to defaults/env)", err)
	}
}

func TestInitViperMalformedConfigIsTolerated(t *testing.T) {
	resetViper(t)
	path := writeConfig(t, ":\tnot: [valid yaml")
	err := InitViper(context.Background(), Service{Name: "app"}, WithConfigFile(path))
	if err != nil {
		t.Fatalf("err = %v, want nil (read errors fall back to defaults/env)", err)
	}
}

func TestInitViperReadsConfigFile(t *testing.T) {
	resetViper(t)
	path := writeConfig(t, "server:\n  host: filehost\n")
	err := InitViper(context.Background(), Service{Name: "app"}, WithConfigFile(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := viper.ConfigFileUsed(); got != path {
		t.Errorf("ConfigFileUsed = %q, want %q", got, path)
	}
	if got := viper.GetString("server.host"); got != "filehost" {
		t.Errorf("server.host = %q, want %q", got, "filehost")
	}
}

func TestInitViperFindsConfigInLookupDirectories(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "appconf.yaml")
	if err := os.WriteFile(path, []byte("found: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := InitViper(context.Background(), Service{Name: "app"},
		WithConfigFileLookupDirectories(dir),
		WithConfigFileName("appconf"),
		WithConfigFileType("yaml"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := viper.ConfigFileUsed(); got != path {
		t.Errorf("ConfigFileUsed = %q, want %q", got, path)
	}
	if !viper.GetBool("found") {
		t.Error("expected key from lookup-directory config to be readable")
	}
}

func TestInitViperDefaultConfigs(t *testing.T) {
	resetViper(t)
	err := InitViper(context.Background(), Service{Name: "app"},
		WithConfigFileLookupDirectories(t.TempDir()),
		WithConfigFileName("missing"),
		WithConfigFileType("yaml"),
		WithDefaultConfigs(map[string]any{"server.port": 8080}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := viper.GetInt("server.port"); got != 8080 {
		t.Errorf("server.port = %d, want 8080 from defaults", got)
	}
}

func TestInitViperEnvOverride(t *testing.T) {
	resetViper(t)
	t.Setenv("MYAPP_SERVER_HOST", "envhost") // prefix + "." -> "_" mapping
	path := writeConfig(t, "server:\n  host: filehost\n")
	err := InitViper(context.Background(), Service{Name: "app"},
		WithConfigFile(path),
		WithEnvPrefix("MYAPP"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := viper.GetString("server.host"); got != "envhost" {
		t.Errorf("server.host = %q, want %q (env overrides config file)", got, "envhost")
	}
}

func TestUnmarshalKeyBackendsFromConfigFile(t *testing.T) {
	resetViper(t)
	t.Setenv("OTLP_HOST", "collector.local")
	t.Setenv("OTLP_TOKEN", "s3cr3t")
	path := writeConfig(t, `
telemetry:
  backends:
    traces:
      endpoint: "${OTLP_HOST}:4317"
      enabled: true
      auth_headers:
        authorization: "Basic ${OTLP_TOKEN}"
    metrics:
      endpoint: 127.0.0.1:4317
      enabled: false
    logs:
      endpoint: 127.0.0.1:4317
      enabled: false
`)
	if err := InitViper(context.Background(), Service{Name: "app"}, WithConfigFile(path)); err != nil {
		t.Fatal(err)
	}

	var b Backends
	if err := UnmarshalKey("telemetry.backends", &b); err != nil {
		t.Fatal(err)
	}
	if got := b.Traces.Endpoint; got != "collector.local:4317" {
		t.Errorf("traces.endpoint = %q, want expanded value", got)
	}
	if !b.Traces.Enabled {
		t.Error("traces.enabled = false, want true")
	}
	if got := b.Traces.AuthHeaders["authorization"]; got != "Basic s3cr3t" {
		t.Errorf("traces auth header = %q, want expanded value", got)
	}
	if b.Metrics.Enabled || b.Logs.Enabled {
		t.Error("metrics/logs should be disabled")
	}

	// Plain viper reads must NOT expand — expansion is a getter-only feature.
	if got := viper.GetString("telemetry.backends.traces.endpoint"); got != "${OTLP_HOST}:4317" {
		t.Errorf("plain viper read = %q, want unexpanded value", got)
	}
}
