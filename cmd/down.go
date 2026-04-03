package cmd

import (
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Revert the last applied migration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		verbose, _ := cmd.Flags().GetBool("verbose")

		reverted, err := runner.Down(cmd.Context())
		if err != nil {
			return err
		}
		if reverted == nil {
			cmd.Println("No migrations have been run yet.")
			return nil
		}

		if verbose {
			cmd.Println("Reverting migration:")
			cmd.Printf("VERSION  NAME\n")
			cmd.Printf("%03d      %s\n", reverted.Version, reverted.Name)
			cmd.Println()
			cmd.Println("Executing SQL:")
			cmd.Println(reverted.DownSQL)
			cmd.Println()
		}

		cmd.Printf("Reverted %03d_%s\n", reverted.Version, reverted.Name)
		return nil
	},
}

func init() {
	downCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "Show detailed output including SQL statements")
	rootCmd.AddCommand(downCmd)
}
