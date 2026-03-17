package cmd

import "github.com/spf13/cobra"

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Revert the last applied migration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		reverted, err := runner.Down(cmd.Context())
		if err != nil {
			return err
		}
		if reverted == nil {
			cmd.Println("No migrations have been run yet.")
			return nil
		}

		cmd.Printf("Reverted %03d_%s\n", reverted.Version, reverted.Name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(downCmd)
}
