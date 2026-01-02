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

func TestNewAgeEncryptor(t *testing.T) {
	encryptor := NewAgeEncryptor()
	if encryptor == nil {
		t.Error("NewAgeEncryptor should not return nil")
	}
}

func TestAgeEncryptor_EncryptDecrypt(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Generate a real test key pair
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}
	testPrivateKey := identity.String()
	testPublicKey := identity.Recipient().String()

	plaintext := []byte("test secret data")

	// Encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{testPublicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want no error", err)
	}

	if len(ciphertext) == 0 {
		t.Error("Ciphertext should not be empty")
	}

	// Decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, testPrivateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v, want no error", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestAgeEncryptor_DecryptMap(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Generate a real test key pair
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}
	testPrivateKey := identity.String()
	testPublicKey := identity.Recipient().String()

	// Test with empty map
	emptyMap := map[string]string{}
	result, err := encryptor.DecryptMap(emptyMap, testPrivateKey)
	if err != nil {
		t.Fatalf("DecryptMap() with empty map error = %v, want no error", err)
	}
	if result == nil {
		t.Error("DecryptMap should return a map, even if empty")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(result))
	}

	// Test with invalid base64
	invalidMap := map[string]string{
		"key1": "invalid-base64!!!",
	}
	_, err = encryptor.DecryptMap(invalidMap, testPrivateKey)
	if err == nil {
		t.Error("DecryptMap should return error for invalid base64")
	}

	// Test with valid base64 but invalid ciphertext
	encryptor2 := NewAgeEncryptor()
	invalidCiphertext := []byte("not-valid-ciphertext")
	invalidMap2 := map[string]string{
		"key1": base64.StdEncoding.EncodeToString(invalidCiphertext),
	}
	_, err = encryptor2.DecryptMap(invalidMap2, testPrivateKey)
	if err == nil {
		t.Error("DecryptMap should return error for invalid ciphertext")
	}

	// Test with valid encrypted data
	plaintext := []byte("test value")
	ciphertext, err := encryptor.Encrypt(plaintext, []string{testPublicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	validMap := map[string]string{
		"key1": base64.StdEncoding.EncodeToString(ciphertext),
	}
	result, err = encryptor.DecryptMap(validMap, testPrivateKey)
	if err != nil {
		t.Fatalf("DecryptMap() with valid data error = %v, want no error", err)
	}
	if string(result["key1"]) != string(plaintext) {
		t.Errorf("Decrypted value doesn't match: got %s, want %s", string(result["key1"]), string(plaintext))
	}
}

func TestAgeEncryptor_Interface(t *testing.T) {
	// Verify that AgeEncryptor implements Encryptor interface
	var _ Encryptor = NewAgeEncryptor()
}
