package cmd

import (
	"fmt"
	"text/tabwriter"

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

		status, err := runner.Status(cmd.Context())
		if err != nil {
			return err
		}
		if len(status) == 0 {
			cmd.Println("No migration files found.")
			return nil
		}

		writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
		_, _ = fmt.Fprintln(writer, "VERSION\tNAME\tSTATE\tAPPLIED_AT")
		for _, entry := range status {
			appliedAt := entry.AppliedAt
			if appliedAt == "" {
				appliedAt = "-"
			}
			_, _ = fmt.Fprintf(writer, "%03d\t%s\t%s\t%s\n", entry.Version, entry.Name, entry.State, appliedAt)
		}

		return writer.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
