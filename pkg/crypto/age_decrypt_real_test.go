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

func TestAgeEncryptor_Decrypt_WithRealKeys(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Generate real age keys for testing
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	plaintext := []byte("test secret data for decryption")

	// Encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey})
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	// Decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, privateKey)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestAgeEncryptor_Decrypt_EmptyIdentity(t *testing.T) {
	encryptor := NewAgeEncryptor()

	ciphertext := []byte("some ciphertext")
	_, err := encryptor.Decrypt(ciphertext, "")
	if err == nil {
		t.Error("Expected error for empty identity")
	}
	if err != nil && err.Error() != "identity (private key) is required" {
		t.Errorf("Expected 'identity (private key) is required' error, got: %v", err)
	}
}

func TestAgeEncryptor_Decrypt_InvalidIdentity(t *testing.T) {
	encryptor := NewAgeEncryptor()

	ciphertext := []byte("some ciphertext")
	_, err := encryptor.Decrypt(ciphertext, "invalid-identity")
	if err == nil {
		t.Error("Expected error for invalid identity")
	}
}

func TestAgeEncryptor_DecryptMap_WithRealKeys(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Generate real age keys for testing
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate identity: %v", err)
	}
	privateKey := identity.String()
	publicKey := identity.Recipient().String()

	// Encrypt multiple values
	encryptedData := map[string]string{
		"key1": base64.StdEncoding.EncodeToString(mustEncrypt(t, encryptor, []byte("value1"), publicKey)),
		"key2": base64.StdEncoding.EncodeToString(mustEncrypt(t, encryptor, []byte("value2"), publicKey)),
		"key3": base64.StdEncoding.EncodeToString(mustEncrypt(t, encryptor, []byte("value3"), publicKey)),
	}

	// Decrypt map
	decryptedMap, err := encryptor.DecryptMap(encryptedData, privateKey)
	if err != nil {
		t.Fatalf("Failed to decrypt map: %v", err)
	}

	if string(decryptedMap["key1"]) != "value1" {
		t.Errorf("Expected key1=value1, got %s", string(decryptedMap["key1"]))
	}
	if string(decryptedMap["key2"]) != "value2" {
		t.Errorf("Expected key2=value2, got %s", string(decryptedMap["key2"]))
	}
	if string(decryptedMap["key3"]) != "value3" {
		t.Errorf("Expected key3=value3, got %s", string(decryptedMap["key3"]))
	}
}

func mustEncrypt(t *testing.T, encryptor *AgeEncryptor, plaintext []byte, publicKey string) []byte {
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey})
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	return ciphertext
}
