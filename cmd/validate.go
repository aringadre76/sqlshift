package cmd

import (
	"fmt"

	"github.com/aringadre76/sqlshift/internal/migration"
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
		if len(issues) == 0 {
			cmd.Println("Validation OK.")
			return nil
		}

		for _, issue := range issues {
			cmd.Printf("%s: %s\n", issue.Severity, issue.Message)
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
