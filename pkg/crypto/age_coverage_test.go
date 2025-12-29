/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crypto

import (
	"encoding/base64"
	"testing"

	"filippo.io/age"
)

func TestAgeEncryptor_Encrypt_EmptyPlaintext(t *testing.T) {
	encryptor := NewAgeEncryptor()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	publicKey := identity.Recipient().String()

	// Encrypt empty plaintext
	ciphertext, err := encryptor.Encrypt([]byte{}, []string{publicKey})
	if err != nil {
		t.Fatalf("Encrypt should handle empty plaintext, got error: %v", err)
	}

	if len(ciphertext) == 0 {
		t.Error("Ciphertext should not be empty even for empty plaintext")
	}
}

func TestAgeEncryptor_Encrypt_EmptyRecipients(t *testing.T) {
	encryptor := NewAgeEncryptor()

	plaintext := []byte("test data")
	_, err := encryptor.Encrypt(plaintext, []string{})

	if err == nil {
		t.Error("Encrypt should return error for empty recipients")
	}
}

func TestAgeEncryptor_Encrypt_InvalidPublicKey(t *testing.T) {
	encryptor := NewAgeEncryptor()

	plaintext := []byte("test data")
	_, err := encryptor.Encrypt(plaintext, []string{"invalid-public-key"})

	if err == nil {
		t.Error("Encrypt should return error for invalid public key")
	}
}

func TestAgeEncryptor_Encrypt_MultipleRecipients(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Generate two identities
	identity1, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity1: %v", err)
	}
	publicKey1 := identity1.Recipient().String()

	identity2, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity2: %v", err)
	}
	publicKey2 := identity2.Recipient().String()

	plaintext := []byte("test secret data")

	// Encrypt with multiple recipients
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey1, publicKey2})
	if err != nil {
		t.Fatalf("Failed to encrypt with multiple recipients: %v", err)
	}

	// Should be decryptable with either key
	decrypted1, err := encryptor.Decrypt(ciphertext, identity1.String())
	if err != nil {
		t.Fatalf("Failed to decrypt with first key: %v", err)
	}
	if string(decrypted1) != string(plaintext) {
		t.Errorf("Decrypted with first key doesn't match: got %s, want %s", string(decrypted1), string(plaintext))
	}

	decrypted2, err := encryptor.Decrypt(ciphertext, identity2.String())
	if err != nil {
		t.Fatalf("Failed to decrypt with second key: %v", err)
	}
	if string(decrypted2) != string(plaintext) {
		t.Errorf("Decrypted with second key doesn't match: got %s, want %s", string(decrypted2), string(plaintext))
	}
}

func TestAgeEncryptor_DecryptMap_PartialFailure(t *testing.T) {
	encryptor := NewAgeEncryptor()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	// Create map with one valid and one invalid entry
	ciphertext, err := encryptor.Encrypt([]byte("value1"), []string{publicKey})
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	validCiphertext := base64.StdEncoding.EncodeToString(ciphertext)
	encryptedData := map[string]string{
		"key1": validCiphertext,
		"key2": "invalid-base64!!!", // Invalid base64
	}

	_, err = encryptor.DecryptMap(encryptedData, privateKey)
	if err == nil {
		t.Error("DecryptMap should return error when some entries are invalid")
	}
}

func TestAgeEncryptor_DecryptMap_ValidBase64InvalidCiphertext(t *testing.T) {
	encryptor := NewAgeEncryptor()

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()

	// Valid base64 but not valid age ciphertext
	validBase64 := base64.StdEncoding.EncodeToString([]byte("not-age-ciphertext"))
	encryptedData := map[string]string{
		"key1": validBase64,
	}

	_, err = encryptor.DecryptMap(encryptedData, privateKey)
	if err == nil {
		t.Error("DecryptMap should return error for valid base64 but invalid ciphertext")
	}
}

// Note: mustEncrypt helper is already defined in age_decrypt_real_test.go
