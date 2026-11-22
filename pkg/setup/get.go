package setup

import (
	"os"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

// Get returns the raw value stored at key with every ${VAR} reference
// expanded from the environment, recursing into slices and maps. It mirrors
// viper.Get; missing keys return nil.
func Get(key string) any {
	return expandEnvValue(viper.Get(key))
}

// GetString behaves like viper.GetString, expanding ${VAR} references first.
func GetString(key string) string {
	return cast.ToString(Get(key))
}

// GetBool behaves like viper.GetBool, expanding ${VAR} references first, so
// a value like "${ENABLED}" resolves before conversion.
func GetBool(key string) bool {
	return cast.ToBool(Get(key))
}

// GetInt behaves like viper.GetInt, expanding ${VAR} references first.
func GetInt(key string) int {
	return cast.ToInt(Get(key))
}

// GetInt64 behaves like viper.GetInt64, expanding ${VAR} references first.
func GetInt64(key string) int64 {
	return cast.ToInt64(Get(key))
}

// GetFloat64 behaves like viper.GetFloat64, expanding ${VAR} references first.
func GetFloat64(key string) float64 {
	return cast.ToFloat64(Get(key))
}

// GetDuration behaves like viper.GetDuration, expanding ${VAR} references
// first, so a value like "${TIMEOUT}" resolves before parsing.
func GetDuration(key string) time.Duration {
	return cast.ToDuration(Get(key))
}

// GetStringSlice behaves like viper.GetStringSlice, expanding ${VAR}
// references in every element first.
func GetStringSlice(key string) []string {
	return cast.ToStringSlice(Get(key))
}

// GetStringMapString behaves like viper.GetStringMapString, expanding ${VAR}
// references in every value first.
func GetStringMapString(key string) map[string]string {
	return cast.ToStringMapString(Get(key))
}

// UnmarshalKey unmarshals the config value stored at key into target,
// expanding ${VAR} references in string values while decoding. It applies
// viper's default decode hooks (string-to-duration, string-to-slice) on top
// of the expansion, so typed struct fields like time.Duration keep working.
func UnmarshalKey(key string, target any) error {
	return viper.UnmarshalKey(key, target, viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			expandEnvHook(),
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
	))
}

// expandEnvValue expands ${VAR} references in every string contained in v,
// recursing into slices and maps. Non-string values pass through unchanged.
func expandEnvValue(v any) any {
	switch t := v.(type) {
	case string:
		return os.ExpandEnv(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = expandEnvValue(e)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = expandEnvValue(e)
		}
		return out
	default:
		return v
	}
}
