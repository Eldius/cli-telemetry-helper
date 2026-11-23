package setup

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveServiceName(t *testing.T) {
	tests := []struct {
		name    string
		svc     Service
		cmdName string
		want    string
	}{
		{"empty name falls back to command name", Service{}, "mycmd", "mycmd"},
		{"explicit name wins", Service{Name: "explicit"}, "mycmd", "explicit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveServiceName(tt.svc, tt.cmdName).Name; got != tt.want {
				t.Errorf("resolveServiceName(%+v, %q).Name = %q, want %q", tt.svc, tt.cmdName, got, tt.want)
			}
		})
	}

	t.Run("empty name and command falls back to executable", func(t *testing.T) {
		if got := resolveServiceName(Service{}, "").Name; got == "" {
			t.Error("expected executable basename fallback, got empty name")
		}
	})
}

func TestSetupAllBackendsDisabled(t *testing.T) {
	shutdown, err := Setup(context.Background(), TelemetryOptions{Service: Service{Name: "test"}})
	if err != nil {
		t.Fatalf("err = %v, want nil for zero-value (all disabled) backends", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown func is nil")
	}
	shutdown() // must be a safe no-op
}

func TestMainWrapperInvokesFunc(t *testing.T) {
	resetViper(t)
	// An empty Service{} and a missing config file must not abort MainWrapper:
	// the service name defaults and missing config is tolerated.
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	called := false
	wantErr := errors.New("m failed")
	err := MainWrapper(context.Background(), Service{}, func(ctx context.Context) error {
		called = true
		if ctx == nil {
			t.Error("nil context passed to m")
		}
		return wantErr
	}, WithConfigFile(missing))

	if !called {
		t.Error("m was not called")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want m's error to propagate", err)
	}
}

// newTestCmd builds a fresh root command (commands are single-use) with args
// and output silenced. missingConfigOpts keeps Run hermetic: without it a
// config file in the real home directory could enable OTLP backends.
func newTestCmd(use string, runE func(*cobra.Command, []string) error) *cobra.Command {
	c := &cobra.Command{
		Use:           use,
		RunE:          runE,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	c.SetArgs([]string{})
	return c
}

func missingConfigOpts(t *testing.T) []ConfigOption {
	t.Helper()
	return []ConfigOption{WithConfigFile(filepath.Join(t.TempDir(), "missing.yaml"))}
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	svc := Service{Name: "run-test"}

	t.Run("success returns 0", func(t *testing.T) {
		resetViper(t)
		cmd := newTestCmd("ok", func(*cobra.Command, []string) error { return nil })
		if code := Run(ctx, svc, cmd, missingConfigOpts(t)...); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})

	t.Run("command error returns 1", func(t *testing.T) {
		resetViper(t)
		cmd := newTestCmd("fail", func(*cobra.Command, []string) error { return errors.New("boom") })
		if code := Run(ctx, svc, cmd, missingConfigOpts(t)...); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})

	t.Run("nil root command returns 1", func(t *testing.T) {
		resetViper(t)
		if code := Run(ctx, svc, nil, missingConfigOpts(t)...); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})

	t.Run("existing hooks are chained in order", func(t *testing.T) {
		resetViper(t)
		var order []string
		cmd := newTestCmd("chained", func(*cobra.Command, []string) error {
			order = append(order, "run")
			return nil
		})
		// Non-E pre variant and E post variant: both must survive wiring.
		cmd.PersistentPreRun = func(*cobra.Command, []string) { order = append(order, "pre") }
		cmd.PersistentPostRunE = func(*cobra.Command, []string) error {
			order = append(order, "post")
			return nil
		}
		if code := Run(ctx, svc, cmd, missingConfigOpts(t)...); code != 0 {
			t.Fatalf("code = %d, want 0", code)
		}
		if want := []string{"pre", "run", "post"}; !reflect.DeepEqual(order, want) {
			t.Errorf("order = %v, want %v", order, want)
		}
	})

	t.Run("failing post hook still runs and returns 1", func(t *testing.T) {
		resetViper(t)
		postRan := false
		cmd := newTestCmd("postfail", func(*cobra.Command, []string) error { return nil })
		cmd.PersistentPostRunE = func(*cobra.Command, []string) error {
			postRan = true
			return errors.New("post boom")
		}
		if code := Run(ctx, svc, cmd, missingConfigOpts(t)...); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
		if !postRan {
			t.Error("consumer post hook did not run")
		}
	})
}

// newBareCmd returns a command with a context set, mirroring what cobra
// guarantees during Execute: the span hooks require a non-nil cmd.Context().
func newBareCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "x"}
	cmd.SetContext(context.Background())
	return cmd
}

func TestInstrumentCmdChaining(t *testing.T) {
	t.Run("E variants are invoked", func(t *testing.T) {
		var calls []string
		cmd := newBareCmd()
		cmd.PersistentPreRunE = func(*cobra.Command, []string) error {
			calls = append(calls, "preE")
			return nil
		}
		cmd.PersistentPostRunE = func(*cobra.Command, []string) error {
			calls = append(calls, "postE")
			return nil
		}
		instrumentCmd(cmd)
		if err := cmd.PersistentPreRunE(cmd, nil); err != nil {
			t.Fatal(err)
		}
		if err := cmd.PersistentPostRunE(cmd, nil); err != nil {
			t.Fatal(err)
		}
		if want := []string{"preE", "postE"}; !reflect.DeepEqual(calls, want) {
			t.Errorf("calls = %v, want %v", calls, want)
		}
	})

	t.Run("pre hook error propagates", func(t *testing.T) {
		wantErr := errors.New("pre boom")
		cmd := newBareCmd()
		cmd.PersistentPreRunE = func(*cobra.Command, []string) error { return wantErr }
		instrumentCmd(cmd)
		if err := cmd.PersistentPreRunE(cmd, nil); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})

	t.Run("post hook error propagates but span hook still runs", func(t *testing.T) {
		wantErr := errors.New("post boom")
		cmd := newBareCmd()
		cmd.PersistentPostRunE = func(*cobra.Command, []string) error { return wantErr }
		instrumentCmd(cmd)
		if err := cmd.PersistentPostRunE(cmd, nil); !errors.Is(err, wantErr) {
			t.Errorf("err = %v, want %v", err, wantErr)
		}
	})
}
