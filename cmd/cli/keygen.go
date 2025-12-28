package main

import (
	"fmt"
	"os"

	"filippo.io/age"
	"github.com/spf13/cobra"
)

func newKeygenCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "keygen",
		Short: "Generate a new age key pair",
		Long: `Generate a new age encryption key pair. This creates a private key file
and displays the corresponding public key. The private key should be kept secure
and never shared. The public key can be shared with your team for encryption.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Generate age identity
			identity, err := age.GenerateX25519Identity()
			if err != nil {
				return fmt.Errorf("failed to generate identity: %w", err)
			}

			// Write private key to file
			if output == "" {
				output = "private-key.age"
			}

			privateKey := identity.String()
			if err := os.WriteFile(output, []byte(privateKey), 0600); err != nil {
				return fmt.Errorf("failed to write private key: %w", err)
			}

			// Display public key
			publicKey := identity.Recipient().String()
			fmt.Fprintf(os.Stdout, "# Public key (share this with your team):\n")
			fmt.Fprintf(os.Stdout, "%s\n", publicKey)
			fmt.Fprintf(os.Stdout, "\n# Private key saved to: %s\n", output)
			fmt.Fprintf(os.Stderr, "⚠️  Keep your private key secure! Never share it.\n")

			return nil
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file for private key (default: private-key.age)")

	return cmd
}

