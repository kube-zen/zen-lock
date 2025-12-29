package main

import (
	"fmt"
	"os"

	"github.com/kube-zen/zen-lock/pkg/crypto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDecryptCmd() *cobra.Command {
	var privkey string
	var input string
	var output string

	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "Decrypt a ZenLock CRD file (debug only)",
		Long: `Decrypt a ZenLock CRD file back to plain text. This is only useful for
local debugging or disaster recovery. The decrypted output should never be
committed to version control.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if privkey == "" {
				return fmt.Errorf("--privkey flag is required")
			}
			if input == "" {
				return fmt.Errorf("--input flag is required")
			}

			// Read private key
			privateKeyData, err := os.ReadFile(privkey)
			if err != nil {
				return fmt.Errorf("failed to read private key file: %w", err)
			}

			// Read input YAML
			inputData, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}

			// Parse ZenLock YAML
			var zenlock map[string]interface{}
			if err := yaml.Unmarshal(inputData, &zenlock); err != nil {
				return fmt.Errorf("failed to parse YAML: %w", err)
			}

			// Extract spec.encryptedData
			spec, ok := zenlock["spec"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid ZenLock: missing 'spec' field")
			}

			encryptedDataRaw, ok := spec["encryptedData"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid ZenLock: missing 'spec.encryptedData' field")
			}

			// Convert to map[string]string
			encryptedData := make(map[string]string)
			for k, v := range encryptedDataRaw {
				val, ok := v.(string)
				if !ok {
					return fmt.Errorf("encrypted value for key %q must be a string", k)
				}
				encryptedData[k] = val
			}

			// Get algorithm from spec (or use default)
			algorithm := crypto.GetDefaultAlgorithm()
			if alg, ok := spec["algorithm"].(string); ok && alg != "" {
				algorithm = alg
			}

			// Validate algorithm is supported
			if !crypto.IsAlgorithmSupported(algorithm) {
				supported := crypto.GetSupportedAlgorithms()
				return fmt.Errorf("unsupported algorithm: %s (supported: %v)", algorithm, supported)
			}

			// Create encryptor for the specified algorithm
			encryptor, err := crypto.CreateEncryptor(algorithm)
			if err != nil {
				return fmt.Errorf("failed to create encryptor: %w", err)
			}

			// Decrypt
			decrypted, err := encryptor.DecryptMap(encryptedData, string(privateKeyData))
			if err != nil {
				return fmt.Errorf("failed to decrypt: %w", err)
			}

			// Construct output YAML
			stringData := make(map[string]string)
			for k, v := range decrypted {
				stringData[k] = string(v)
			}

			outputObj := map[string]interface{}{
				"stringData": stringData,
			}

			// Marshal to YAML
			outputData, err := yaml.Marshal(outputObj)
			if err != nil {
				return fmt.Errorf("failed to marshal YAML: %w", err)
			}

			// Write output
			if output == "" {
				fmt.Fprint(os.Stdout, string(outputData))
			} else {
				if err := os.WriteFile(output, outputData, 0600); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Fprintf(os.Stderr, "✅ Decrypted secret written to: %s\n", output)
				fmt.Fprintf(os.Stderr, "⚠️  Keep this file secure! Never commit it to version control.\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&privkey, "privkey", "k", "", "Private key file (required)")
	cmd.Flags().StringVarP(&input, "input", "i", "", "Input ZenLock YAML file (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")

	return cmd
}
