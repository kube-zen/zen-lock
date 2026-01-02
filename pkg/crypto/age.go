package crypto

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"filippo.io/age"

	"github.com/kube-zen/zen-lock/pkg/config"
	"github.com/kube-zen/zen-lock/pkg/controller/metrics"
)

// AgeEncryptor implements Encryptor using age encryption
type AgeEncryptor struct{}

// NewAgeEncryptor creates a new AgeEncryptor instance
func NewAgeEncryptor() *AgeEncryptor {
	return &AgeEncryptor{}
}

// Encrypt encrypts plaintext using age with the provided recipients (public keys)
func (a *AgeEncryptor) Encrypt(plaintext []byte, recipients []string) ([]byte, error) {
	if len(recipients) == 0 {
		return nil, fmt.Errorf("at least one recipient (public key) is required")
	}

	// Parse recipients
	ageRecipients := make([]age.Recipient, 0, len(recipients))
	for _, r := range recipients {
		recipient, err := age.ParseX25519Recipient(r)
		if err != nil {
			return nil, fmt.Errorf("failed to parse recipient %q: %w", r, err)
		}
		ageRecipients = append(ageRecipients, recipient)
	}

	// Encrypt the data
	var encrypted bytes.Buffer
	w, err := age.Encrypt(&encrypted, ageRecipients...)
	if err != nil {
		metrics.RecordAlgorithmError(config.DefaultAlgorithm, "encryption_failed")
		return nil, fmt.Errorf("failed to create encrypt writer: %w", err)
	}

	if _, err := w.Write(plaintext); err != nil {
		metrics.RecordAlgorithmError(config.DefaultAlgorithm, "encryption_failed")
		return nil, fmt.Errorf("failed to write plaintext: %w", err)
	}

	if err := w.Close(); err != nil {
		metrics.RecordAlgorithmError(config.DefaultAlgorithm, "encryption_failed")
		return nil, fmt.Errorf("failed to close encrypt writer: %w", err)
	}

	// Record successful encryption
	metrics.RecordAlgorithmUsage(config.DefaultAlgorithm, "encrypt")

	return encrypted.Bytes(), nil
}

// Decrypt decrypts ciphertext using age with the provided identity (private key)
func (a *AgeEncryptor) Decrypt(ciphertext []byte, identity string) ([]byte, error) {
	if identity == "" {
		return nil, fmt.Errorf("identity (private key) is required")
	}

	// Parse identity
	id, err := age.ParseX25519Identity(identity)
	if err != nil {
		return nil, fmt.Errorf("failed to parse identity: %w", err)
	}

	// Decrypt the data
	r, err := age.Decrypt(bytes.NewReader(ciphertext), id)
	if err != nil {
		metrics.RecordAlgorithmError(config.DefaultAlgorithm, "decryption_failed")
		return nil, fmt.Errorf("failed to create decrypt reader: %w", err)
	}

	decrypted, err := io.ReadAll(r)
	if err != nil {
		metrics.RecordAlgorithmError(config.DefaultAlgorithm, "decryption_failed")
		return nil, fmt.Errorf("failed to read decrypted data: %w", err)
	}

	// Record successful decryption
	metrics.RecordAlgorithmUsage(config.DefaultAlgorithm, "decrypt")

	return decrypted, nil
}

// DecryptMap decrypts a map of base64-encoded encrypted values
func (a *AgeEncryptor) DecryptMap(encryptedData map[string]string, identity string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(encryptedData))

	for key, encryptedValue := range encryptedData {
		// Decode base64
		ciphertext, err := base64.StdEncoding.DecodeString(encryptedValue)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 for key %q: %w", key, err)
		}

		// Decrypt
		plaintext, err := a.Decrypt(ciphertext, identity)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt key %q: %w", key, err)
		}

		result[key] = plaintext
	}

	return result, nil
}
