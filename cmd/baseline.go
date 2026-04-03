package cmd

import (
	"fmt"
	"strconv"

	"github.com/aringadre76/sqlshift/internal/db"
	"github.com/aringadre76/sqlshift/internal/migration"
	"github.com/spf13/cobra"
)

var baselineCmd = &cobra.Command{
	Use:   "baseline <version>",
	Short: "Mark the database at a specific version (for brownfield projects)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := currentConfig(cmd)
		if err != nil {
			return err
		}

		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		versionStr := args[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil || version <= 0 {
			return fmt.Errorf("invalid version %q: must be a positive integer", versionStr)
		}

		// Verify the migration file exists for this version
		migrations, err := migration.LoadDir(cfg.MigrationsDir)
		if err != nil {
			return fmt.Errorf("loading migrations: %w", err)
		}

		var found *migration.Migration
		for _, m := range migrations {
			if m.Version == version {
				found = m
				break
			}
		}

		if found == nil {
			return fmt.Errorf("migration %03d not found in %s", version, cfg.MigrationsDir)
		}

		// Create history table if it doesn't exist
		if err := runner.Dialect.CreateHistoryTable(cmd.Context(), runner.DB, runner.TableName); err != nil {
			return fmt.Errorf("creating history table: %w", err)
		}

		// Get applied migrations to check if this version is already applied
		applied, err := runner.Dialect.GetApplied(cmd.Context(), runner.DB, runner.TableName)
		if err != nil {
			return fmt.Errorf("querying applied migrations: %w", err)
		}

		for _, record := range applied {
			if record.Version == version {
				cmd.Printf("Migration %03d (%s) is already marked as applied\n", version, found.Name)
				return nil
			}
			if record.Version > version {
				return fmt.Errorf("cannot baseline at version %03d: there are migrations already applied (version %03d)", version, record.Version)
			}
		}

		// Get the current highest applied version
		highestApplied := 0
		for _, record := range applied {
			if record.Version > highestApplied {
				highestApplied = record.Version
			}
		}

		if highestApplied >= version {
			return fmt.Errorf("cannot baseline: database already has migrations applied up to version %03d", highestApplied)
		}

		// Insert the baseline record
		// Need to use a transaction for the dialect method
		tx, err := runner.DB.BeginTx(cmd.Context(), nil)
		if err != nil {
			return fmt.Errorf("beginning transaction: %w", err)
		}

		if err := runner.Dialect.InsertMigration(cmd.Context(), tx, runner.TableName, db.AppliedMigration{
			Version:  version,
			Name:     found.Name,
			Checksum: found.Checksum,
		}); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("inserting baseline record: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing baseline: %w", err)
		}

		cmd.Printf("Baseline complete: marked database at version %03d (%s)\n", version, found.Name)
		cmd.Println("All migrations up to this version are now considered applied.")
		cmd.Println("Subsequent 'sqlshift up' will apply migrations with higher versions.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(baselineCmd)
}
