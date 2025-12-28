package main

import (
	"fmt"
	"os"

	"filippo.io/age"
	"github.com/spf13/cobra"
)

func newPubkeyCmd() *cobra.Command {
	var input string

	cmd := &cobra.Command{
		Use:   "pubkey",
		Short: "Extract the public key from a private key file",
		Long: `Extract and display the public key from a private key file.
The public key can be shared with your team for encryption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return fmt.Errorf("--input flag is required")
			}

			// Read private key
			privateKeyData, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("failed to read private key file: %w", err)
			}

			// Parse identity
			identity, err := age.ParseX25519Identity(string(privateKeyData))
			if err != nil {
				return fmt.Errorf("failed to parse private key: %w", err)
			}

			// Display public key
			publicKey := identity.Recipient().String()
			fmt.Fprintf(os.Stdout, "%s\n", publicKey)

			return nil
		},
	}

	cmd.Flags().StringVarP(&input, "input", "i", "", "Input file containing private key (required)")

	return cmd
}
