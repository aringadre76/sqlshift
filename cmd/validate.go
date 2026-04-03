package cmd

import (
	"fmt"

	"github.com/aringadre76/sqlshift/internal/migration"
	"github.com/aringadre76/sqlshift/internal/output"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate migrations on disk against history",
	RunE: func(cmd *cobra.Command, _ []string) error {
		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		issues, err := runner.Validate(cmd.Context())
		if err != nil {
			return err
		}

		// Convert to output format
		outputIssues := make([]output.ValidationIssue, len(issues))
		for i, issue := range issues {
			outputIssues[i] = output.ValidationIssue{
				Severity: issue.Severity,
				Message:  issue.Message,
			}
		}

		format, _ := cmd.Flags().GetString("output")
		if err := output.PrintValidate(cmd.OutOrStdout(), format, outputIssues); err != nil {
			return fmt.Errorf("printing output: %w", err)
		}

		if migration.HasValidationErrors(issues) {
			return fmt.Errorf("validation failed")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
