package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize sqlshift in the current directory",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := currentConfig(cmd)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(cfg.MigrationsDir, 0o755); err != nil {
			return fmt.Errorf("creating migrations directory %s: %w", cfg.MigrationsDir, err)
		}

		if _, err := os.Stat(".shift.toml"); os.IsNotExist(err) {
			contents := fmt.Sprintf(
				"database_url = \"\"\nmigrations_dir = %q\ntable_name = %q\n",
				cfg.MigrationsDir,
				cfg.TableName,
			)
			if writeErr := os.WriteFile(".shift.toml", []byte(contents), 0o644); writeErr != nil {
				return fmt.Errorf("writing .shift.toml: %w", writeErr)
			}
		}

		cmd.Printf("Initialized sqlshift with migrations directory %s\n", cfg.MigrationsDir)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
