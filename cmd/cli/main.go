package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "0.1.0-alpha"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "zen-lock",
		Short: "zen-lock - Zero-Knowledge secret manager for Kubernetes",
		Long: `zen-lock is a Kubernetes-native secret manager that implements Zero-Knowledge secret storage.

ZenLock CRDs store only ciphertext (source-of-truth). During Pod injection, zen-lock decrypts in the webhook and creates an ephemeral Kubernetes Secret containing plaintext, which is mounted into the Pod. Protect ephemeral Secrets via RBAC and etcd encryption-at-rest.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate),
	}

	rootCmd.AddCommand(newKeygenCmd())
	rootCmd.AddCommand(newPubkeyCmd())
	rootCmd.AddCommand(newEncryptCmd())
	rootCmd.AddCommand(newDecryptCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
