package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/kube-zen/zen-lock/pkg/crypto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEncryptCmd() *cobra.Command {
	var pubkey string
	var input string
	var output string

	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "Encrypt a YAML file containing secret data",
		Long: `Encrypt a YAML file containing secret data. The input file should have a
'stringData' field with key-value pairs. The output will be a ZenLock CRD YAML
file with encrypted data that can be safely committed to Git.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if pubkey == "" {
				return fmt.Errorf("--pubkey flag is required")
			}
			if input == "" {
				return fmt.Errorf("--input flag is required")
			}

			// Read input YAML
			inputData, err := os.ReadFile(input)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}

			// Parse YAML
			var obj map[string]interface{}
			if err := yaml.Unmarshal(inputData, &obj); err != nil {
				return fmt.Errorf("failed to parse YAML: %w", err)
			}

			// Extract stringData
			stringData, ok := obj["stringData"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("input YAML must contain a 'stringData' field with key-value pairs")
			}

			// Initialize encryptor
			encryptor := crypto.NewAgeEncryptor()

			// Encrypt each value
			encryptedData := make(map[string]string)
			for k, v := range stringData {
				val, ok := v.(string)
				if !ok {
					return fmt.Errorf("value for key %q must be a string", k)
				}

				// Encrypt
				ciphertext, err := encryptor.Encrypt([]byte(val), []string{pubkey})
				if err != nil {
					return fmt.Errorf("failed to encrypt key %q: %w", k, err)
				}

				// Base64 encode for storage
				encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
			}

			// Construct ZenLock CRD
			zenlock := map[string]interface{}{
				"apiVersion": "security.zen.io/v1alpha1",
				"kind":       "ZenLock",
				"metadata": map[string]interface{}{
					"name": obj["metadata"].(map[string]interface{})["name"],
				},
				"spec": map[string]interface{}{
					"encryptedData": encryptedData,
					"algorithm":     "age",
				},
			}

			// Add namespace if present
			if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
				if ns, ok := metadata["namespace"].(string); ok {
					zenlock["metadata"].(map[string]interface{})["namespace"] = ns
				}
			}

			// Marshal to YAML
			outputData, err := yaml.Marshal(zenlock)
			if err != nil {
				return fmt.Errorf("failed to marshal YAML: %w", err)
			}

			// Write output
			if output == "" {
				fmt.Fprint(os.Stdout, string(outputData))
			} else {
				if err := os.WriteFile(output, outputData, 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Fprintf(os.Stderr, "✅ Encrypted secret written to: %s\n", output)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&pubkey, "pubkey", "p", "", "Public key for encryption (required)")
	cmd.Flags().StringVarP(&input, "input", "i", "", "Input YAML file with stringData (required)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file (default: stdout)")

	return cmd
}

