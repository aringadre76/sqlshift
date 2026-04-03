package cmd

import (
	"github.com/aringadre76/sqlshift/internal/output"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show applied and pending migrations",
	RunE: func(cmd *cobra.Command, _ []string) error {
		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		entries, err := runner.Status(cmd.Context())
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			cmd.Println("No migration files found.")
			return nil
		}

		// Convert to output format
		migrations := make([]output.MigrationInfo, len(entries))
		for i, e := range entries {
			migrations[i] = output.MigrationInfo{
				Version:   e.Version,
				Name:      e.Name,
				State:     e.State,
				AppliedAt: e.AppliedAt,
			}
		}

		format, _ := cmd.Flags().GetString("output")
		return output.PrintStatus(cmd.OutOrStdout(), format, migrations)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
