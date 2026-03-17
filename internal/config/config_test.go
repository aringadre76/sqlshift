package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("defaults when config missing", func(t *testing.T) {
		cfg, err := Load(LoadOptions{
			ConfigFile: filepath.Join(t.TempDir(), ".shift.toml"),
		})
		require.NoError(t, err)
		require.Equal(t, "./migrations", cfg.MigrationsDir)
		require.Equal(t, "shift_migrations", cfg.TableName)
		require.Empty(t, cfg.DatabaseURL)
	})

	t.Run("loads values from config file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".shift.toml")
		err := os.WriteFile(path, []byte(`
database_url = "sqlite://local.db"
migrations_dir = "./db/migrations"
table_name = "custom_history"
`), 0o644)
		require.NoError(t, err)

		cfg, loadErr := Load(LoadOptions{ConfigFile: path})
		require.NoError(t, loadErr)
		require.Equal(t, "sqlite://local.db", cfg.DatabaseURL)
		require.Equal(t, "./db/migrations", cfg.MigrationsDir)
		require.Equal(t, "custom_history", cfg.TableName)
	})

	t.Run("env overrides config values", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".shift.toml")
		err := os.WriteFile(path, []byte(`
database_url = "sqlite://from-file.db"
migrations_dir = "./from-file"
table_name = "from_file"
`), 0o644)
		require.NoError(t, err)

		t.Setenv("SHIFT_DATABASE_URL", "postgres://env")
		t.Setenv("SHIFT_MIGRATIONS_DIR", "./from-env")
		t.Setenv("SHIFT_TABLE_NAME", "from_env")

		cfg, loadErr := Load(LoadOptions{ConfigFile: path})
		require.NoError(t, loadErr)
		require.Equal(t, "postgres://env", cfg.DatabaseURL)
		require.Equal(t, "./from-env", cfg.MigrationsDir)
		require.Equal(t, "from_env", cfg.TableName)
	})

	t.Run("cli overrides env and file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".shift.toml")
		err := os.WriteFile(path, []byte(`
database_url = "sqlite://from-file.db"
migrations_dir = "./from-file"
table_name = "from_file"
`), 0o644)
		require.NoError(t, err)

		t.Setenv("SHIFT_DATABASE_URL", "postgres://env")
		t.Setenv("SHIFT_MIGRATIONS_DIR", "./from-env")
		t.Setenv("SHIFT_TABLE_NAME", "from_env")

		cfg, loadErr := Load(LoadOptions{
			ConfigFile:    path,
			DatabaseURL:   "mysql://cli",
			MigrationsDir: "./from-cli",
			TableName:     "from_cli",
		})
		require.NoError(t, loadErr)
		require.Equal(t, "mysql://cli", cfg.DatabaseURL)
		require.Equal(t, "./from-cli", cfg.MigrationsDir)
		require.Equal(t, "from_cli", cfg.TableName)
	})
}

func TestValidateTableName(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		wantErr   bool
	}{
		{name: "default", tableName: "shift_migrations", wantErr: false},
		{name: "snake_case", tableName: "custom_history_table", wantErr: false},
		{name: "starts with number", tableName: "1bad", wantErr: true},
		{name: "has space", tableName: "bad name", wantErr: true},
		{name: "has dash", tableName: "bad-name", wantErr: true},
		{name: "has punctuation", tableName: "bad;", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTableName(tt.tableName)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
