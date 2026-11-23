package setup

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestExpandEnvValue(t *testing.T) {
	t.Setenv("EXPAND_ME", "expanded")

	tests := []struct {
		name string
		in   any
		want any
	}{
		{"string", "prefix-${EXPAND_ME}", "prefix-expanded"},
		{"string without refs", "plain", "plain"},
		{"unset var becomes empty", "${UNSET_VAR_12345}", ""},
		{"int passthrough", 42, 42},
		{"nil", nil, nil},
		{"slice", []any{"${EXPAND_ME}", 1}, []any{"expanded", 1}},
		{
			"map",
			map[string]any{"k": "${EXPAND_ME}", "n": 2},
			map[string]any{"k": "expanded", "n": 2},
		},
		{
			"nested slice in map in slice",
			[]any{map[string]any{"list": []any{"${EXPAND_ME}"}}},
			[]any{map[string]any{"list": []any{"expanded"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandEnvValue(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expandEnvValue(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestGettersExpandEnvRefs(t *testing.T) {
	resetViper(t)
	t.Setenv("G_HOST", "example.com")
	t.Setenv("G_PORT", "9090")
	t.Setenv("G_FLAG", "true")
	t.Setenv("G_TIMEOUT", "5s")

	viper.Set("server.host", "${G_HOST}:8080")
	viper.Set("server.port", "${G_PORT}")
	viper.Set("server.flag", "${G_FLAG}")
	viper.Set("server.timeout", "${G_TIMEOUT}")
	viper.Set("server.tags", []any{"${G_HOST}", "plain"})
	viper.Set("server.labels", map[string]any{"host": "${G_HOST}", "env": "dev"})
	viper.Set("plain.int", 42)

	tests := []struct {
		name string
		key  string
		get  func(string) any
		want any
	}{
		{"GetString", "server.host", func(k string) any { return GetString(k) }, "example.com:8080"},
		{"GetInt", "server.port", func(k string) any { return GetInt(k) }, 9090},
		{"GetInt64", "server.port", func(k string) any { return GetInt64(k) }, int64(9090)},
		{"GetFloat64", "server.port", func(k string) any { return GetFloat64(k) }, 9090.0},
		{"GetBool", "server.flag", func(k string) any { return GetBool(k) }, true},
		{"GetDuration", "server.timeout", func(k string) any { return GetDuration(k) }, 5 * time.Second},
		{"GetStringSlice", "server.tags", func(k string) any { return GetStringSlice(k) }, []string{"example.com", "plain"}},
		{"GetStringMapString", "server.labels", func(k string) any { return GetStringMapString(k) },
			map[string]string{"host": "example.com", "env": "dev"}},
		{"non-string passthrough", "plain.int", func(k string) any { return GetInt(k) }, 42},
		{"missing key", "missing.key", func(k string) any { return GetString(k) }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.get(tt.key); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("get(%q) = %v (%T), want %v (%T)", tt.key, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestGetExpandsNestedStructures(t *testing.T) {
	resetViper(t)
	t.Setenv("G_DEEP", "deep-value")

	viper.Set("deep", map[string]any{
		"list":  []any{"${G_DEEP}"},
		"inner": map[string]any{"key": "${G_DEEP}"},
	})
	want := map[string]any{
		"list":  []any{"deep-value"},
		"inner": map[string]any{"key": "deep-value"},
	}
	if got := Get("deep"); !reflect.DeepEqual(got, want) {
		t.Errorf("Get(deep) = %v, want %v", got, want)
	}
}

func TestUnmarshalKeyTypedFields(t *testing.T) {
	resetViper(t)
	t.Setenv("G_TMO", "250ms")
	t.Setenv("G_TAGS", "a,b,c")
	t.Setenv("G_PORT", "8080")

	type serverCfg struct {
		Timeout time.Duration `mapstructure:"timeout"`
		Tags    []string      `mapstructure:"tags"`
		Port    int           `mapstructure:"port"`
	}
	viper.Set("srv", map[string]any{
		"timeout": "${G_TMO}",
		"tags":    "${G_TAGS}",
		"port":    "${G_PORT}",
	})

	var cfg serverCfg
	if err := UnmarshalKey("srv", &cfg); err != nil {
		t.Fatal(err)
	}
	want := serverCfg{Timeout: 250 * time.Millisecond, Tags: []string{"a", "b", "c"}, Port: 8080}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("UnmarshalKey = %+v, want %+v", cfg, want)
	}
}
