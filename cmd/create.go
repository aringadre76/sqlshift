package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aringadre76/sqlshift/internal/migration"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create the next numbered migration file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := currentConfig(cmd)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(cfg.MigrationsDir, 0o755); err != nil {
			return fmt.Errorf("creating migrations directory %s: %w", cfg.MigrationsDir, err)
		}

		name := sanitizeMigrationName(args[0])
		if name == "" {
			return fmt.Errorf("migration name must contain at least one letter or number")
		}

		nextVersion, err := nextMigrationVersion(cfg.MigrationsDir)
		if err != nil {
			return fmt.Errorf("determining next migration version: %w", err)
		}

		baseName := fmt.Sprintf("%03d_%s.sql", nextVersion, name)
		path := filepath.Join(cfg.MigrationsDir, baseName)
		for suffix := 2; fileExists(path); suffix++ {
			path = filepath.Join(cfg.MigrationsDir, fmt.Sprintf("%03d_%s_%d.sql", nextVersion, name, suffix))
		}

		contents := "-- shift:up\n\n-- shift:down\n"
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			return fmt.Errorf("creating migration file %s: %w", path, err)
		}

		// stdout so shell scripts can capture the path (Cobra may route cmd.Println to stderr in some setups).
		_, _ = fmt.Fprintln(os.Stdout, path)
		return nil
	},
}

var nonWordPattern = regexp.MustCompile(`[^a-z0-9]+`)

func init() {
	rootCmd.AddCommand(createCmd)
}

func nextMigrationVersion(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading migrations directory %s: %w", dir, err)
	}

	maxVersion := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		version, _, err := migration.ParseFilename(entry.Name())
		if err != nil {
			return 0, fmt.Errorf("parsing migration filename %s: %w", entry.Name(), err)
		}
		if version > maxVersion {
			maxVersion = version
		}
	}

	return maxVersion + 1, nil
}

func sanitizeMigrationName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = nonWordPattern.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	return name
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
