package cmd

import (
	"context"
	"fmt"

	"github.com/aringadre76/sqlshift/internal/config"
	"github.com/spf13/cobra"
)

type configContextKey struct{}

var (
	configFileFlag    string
	databaseURLFlag   string
	migrationsDirFlag string
	tableNameFlag     string
	dryRunFlag        bool
	verboseFlag       bool
)

var rootCmd = &cobra.Command{
	Use:          "sqlshift",
	Short:        "SQL-first database migration CLI",
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(config.LoadOptions{
			ConfigFile:    configFileFlag,
			DatabaseURL:   databaseURLFlag,
			MigrationsDir: migrationsDirFlag,
			TableName:     tableNameFlag,
		})
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		cmd.SetContext(context.WithValue(cmd.Context(), configContextKey{}, cfg))
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configFileFlag, "config", "", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&databaseURLFlag, "database-url", "", "Database connection URL")
	rootCmd.PersistentFlags().StringVar(&migrationsDirFlag, "migrations-dir", "", "Directory containing SQL migrations")
	rootCmd.PersistentFlags().StringVar(&tableNameFlag, "table-name", "", "Schema history table name")
	rootCmd.PersistentFlags().BoolVar(&dryRunFlag, "dry-run", false, "Preview migrations without applying")
	rootCmd.PersistentFlags().BoolVar(&verboseFlag, "verbose", false, "Show detailed output including SQL statements")
	rootCmd.PersistentFlags().String("output", "table", "Output format: table or json")
}

func currentConfig(cmd *cobra.Command) (config.Config, error) {
	cfg, ok := cmd.Context().Value(configContextKey{}).(config.Config)
	if !ok {
		return config.Config{}, fmt.Errorf("config not loaded")
	}

	return cfg, nil
}
