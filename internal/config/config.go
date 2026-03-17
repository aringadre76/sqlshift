package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/viper"
)

const (
	DefaultMigrationsDir = "./migrations"
	DefaultTableName     = "shift_migrations"
)

var tableNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Config struct {
	DatabaseURL   string `mapstructure:"database_url"`
	MigrationsDir string `mapstructure:"migrations_dir"`
	TableName     string `mapstructure:"table_name"`
}

type LoadOptions struct {
	ConfigFile    string
	DatabaseURL   string
	MigrationsDir string
	TableName     string
}

func Load(options LoadOptions) (Config, error) {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetDefault("migrations_dir", DefaultMigrationsDir)
	v.SetDefault("table_name", DefaultTableName)

	if options.ConfigFile != "" {
		v.SetConfigFile(options.ConfigFile)
	} else {
		v.SetConfigName(".shift")
		v.AddConfigPath(".")
	}

	if err := v.BindEnv("database_url", "SHIFT_DATABASE_URL"); err != nil {
		return Config{}, fmt.Errorf("binding database_url env: %w", err)
	}
	if err := v.BindEnv("migrations_dir", "SHIFT_MIGRATIONS_DIR"); err != nil {
		return Config{}, fmt.Errorf("binding migrations_dir env: %w", err)
	}
	if err := v.BindEnv("table_name", "SHIFT_TABLE_NAME"); err != nil {
		return Config{}, fmt.Errorf("binding table_name env: %w", err)
	}
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var configFileNotFound viper.ConfigFileNotFoundError
		if !os.IsNotExist(err) && !errors.As(err, &configFileNotFound) {
			return Config{}, fmt.Errorf("reading config: %w", err)
		}
	}

	if options.DatabaseURL != "" {
		v.Set("database_url", options.DatabaseURL)
	}
	if options.MigrationsDir != "" {
		v.Set("migrations_dir", options.MigrationsDir)
	}
	if options.TableName != "" {
		v.Set("table_name", options.TableName)
	}

	cfg := Config{
		DatabaseURL:   v.GetString("database_url"),
		MigrationsDir: v.GetString("migrations_dir"),
		TableName:     v.GetString("table_name"),
	}

	if err := validateTableName(cfg.TableName); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validateTableName(name string) error {
	if !tableNamePattern.MatchString(name) {
		return fmt.Errorf("invalid table name %q: must match [A-Za-z_][A-Za-z0-9_]*", name)
	}

	return nil
}
