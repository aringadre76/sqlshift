package cmd

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var repairCmd = &cobra.Command{
	Use:   "repair <version> <checksum>",
	Short: "Fix a migration checksum mismatch",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner, cleanup, err := openRunner(cmd)
		if err != nil {
			return err
		}
		defer cleanup()

		version := args[0]
		newChecksum := args[1]

		// Validate version is a number
		var versionNum int
		_, err = fmt.Sscanf(version, "%d", &versionNum)
		if err != nil || versionNum <= 0 {
			return fmt.Errorf("invalid version %q: must be a positive integer", version)
		}

		// Validate checksum is hex
		newChecksum = strings.TrimSpace(newChecksum)
		if len(newChecksum) != 64 {
			return fmt.Errorf("invalid checksum length %d: expected 64 characters (SHA-256 hex)", len(newChecksum))
		}

		// Verify checksum is valid hex
		_, err = hex.DecodeString(newChecksum)
		if err != nil {
			return fmt.Errorf("invalid checksum %q: must be valid SHA-256 hex string", newChecksum)
		}

		// Update the checksum in the database
		err = runner.UpdateChecksum(cmd.Context(), versionNum, newChecksum)
		if err != nil {
			return fmt.Errorf("updating checksum: %w", err)
		}

		cmd.Printf("Fixed checksum for migration %03d\n", versionNum)
		cmd.Printf("New checksum: %s\n", newChecksum)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(repairCmd)
}
