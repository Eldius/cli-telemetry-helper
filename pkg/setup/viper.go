package setup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	// ErrEmptyAppName is returned by InitViper when the service name is empty.
	ErrEmptyAppName = errors.New("empty service name")
	// ErrViperInitialization wraps any failure to read the configuration file.
	ErrViperInitialization = errors.New("viper initialization error")
)

// ConfigOptions customizes how InitViper locates and reads configuration.
// Set fields through the With* option functions rather than directly.
type ConfigOptions struct {
	DefaultConfigs              map[string]any
	ConfigFileLookupDirectories []string
	EnvPrefix                   string
	ConfigFile                  string
	ConfigFileName              string
	ConfigFileType              string
}

// ConfigOption mutates ConfigOptions; constructed by the With* functions.
type ConfigOption func(*ConfigOptions)

// WithConfigFileLookupDirectories defines directories to lookup for
// config file
func WithConfigFileLookupDirectories(dirs ...string) ConfigOption {
	return func(v *ConfigOptions) {
		v.ConfigFileLookupDirectories = dirs
	}
}

// WithDefaultConfigs defines default Viper values
func WithDefaultConfigs(configs map[string]any) ConfigOption {
	return func(v *ConfigOptions) {
		v.DefaultConfigs = configs
	}
}

// WithEnvPrefix defines the environment variables'
// name prefix for properties lookup
func WithEnvPrefix(prefix string) ConfigOption {
	return func(v *ConfigOptions) {
		v.EnvPrefix = prefix
	}
}

// WithConfigFile sets an explicit config file path, skipping directory lookup.
func WithConfigFile(path string) ConfigOption {
	return func(v *ConfigOptions) {
		v.ConfigFile = path
	}
}

// WithConfigFileName sets the config file name (without extension) searched
// for in the lookup directories.
func WithConfigFileName(name string) ConfigOption {
	return func(v *ConfigOptions) {
		v.ConfigFileName = name
	}
}

// WithConfigFileType sets the config file format (e.g. "yaml", "json").
func WithConfigFileType(t string) ConfigOption {
	return func(v *ConfigOptions) {
		v.ConfigFileType = t
	}
}

// InitViper configures the global viper instance: it applies opts, enables
// environment variable overrides (mapping "." in keys to "_" with the
// configured prefix), and reads the config file. Unless WithConfigFile is
// used, the file is searched in the WithConfigFileLookupDirectories
// directories, then in ~/.<service-name>/ and ~/. It returns
// ErrViperInitialization when no config file can be read.
func InitViper(ctx context.Context, svc Service, opts ...ConfigOption) error {
	cfg := ConfigOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}

	if svc.Name == "" {
		return fmt.Errorf("invalid app name: %w", ErrEmptyAppName)
	}

	if cfg.ConfigFile != "" {
		// Use the config file from the flag.
		viper.SetConfigFile(cfg.ConfigFile)
	} else {
		// Find a home directory.
		home, err := homedir.Dir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}

		for _, dir := range cfg.ConfigFileLookupDirectories {
			viper.AddConfigPath(dir)
		}
		viper.AddConfigPath(filepath.Join(home, fmt.Sprintf(".%s", svc.Name)))
		viper.AddConfigPath(filepath.Join(home))
		viper.SetConfigName(cfg.ConfigFileName)
		viper.SetConfigType(cfg.ConfigFileType)
	}

	for k, v := range cfg.DefaultConfigs {
		viper.SetDefault(k, v)
	}
	viper.SetEnvPrefix(cfg.EnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("%w: %w", ErrViperInitialization, err)
	}
	slog.Info("Using config file", "path", viper.ConfigFileUsed())
	return nil
}
