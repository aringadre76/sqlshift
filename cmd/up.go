package cmd

import (
	"fmt"
	"text/tabwriter"

	dbpkg "github.com/aringadre76/sqlshift/internal/db"
	"github.com/aringadre76/sqlshift/internal/migration"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE: func(cmd *cobra.Command, _ []string) error {
		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			return runUpDryRun(cmd, runner)
		}

		verbose, _ := cmd.Flags().GetBool("verbose")
		applied, err := runner.UpWithVerbose(cmd.Context(), verbose)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			cmd.Println("No pending migrations.")
			return nil
		}

		if verbose {
			cmd.Println("Applying migrations:")
		}
		writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
		_, _ = fmt.Fprintln(writer, "VERSION\tNAME")
		for _, entry := range applied {
			_, _ = fmt.Fprintf(writer, "%03d\t%s\n", entry.Version, entry.Name)
		}
		return writer.Flush()
	},
}

func runUpDryRun(cmd *cobra.Command, runner *migration.Runner) error {
	plan, err := runner.PlanUp(cmd.Context())
	if err != nil {
		return err
	}
	if len(plan) == 0 {
		cmd.Println("No pending migrations.")
		return nil
	}

	cmd.Println("Pending migrations (dry run):")
	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
	_, _ = fmt.Fprintln(writer, "VERSION\tNAME")
	for _, entry := range plan {
		_, _ = fmt.Fprintf(writer, "%03d\t%s\n", entry.Version, entry.Name)
	}
	return writer.Flush()
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func openRunner(cmd *cobra.Command) (*migration.Runner, func(), error) {
	cfg, err := currentConfig(cmd)
	if err != nil {
		return nil, nil, err
	}
	if cfg.DatabaseURL == "" {
		return nil, nil, fmt.Errorf("database_url is required")
	}

	database, dialect, err := dbpkg.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() {
		_ = database.Close()
	}

	return migration.NewRunner(database, dialect, cfg.MigrationsDir, cfg.TableName), cleanup, nil
}
