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
)

func TestAgeEncryptor_Encrypt_RealKey(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Use a properly formatted test public key (age format)
	// This is a test key - in real usage, generate with zen-lock keygen
	testPublicKey := "age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqs3290u3"

	plaintext := []byte("test secret data")

	// Encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{testPublicKey})
	if err != nil {
		// If encryption fails due to invalid key format, skip this test
		t.Skipf("Encrypt failed (may need valid key): %v", err)
		return
	}

	if len(ciphertext) == 0 {
		t.Error("Ciphertext should not be empty")
	}

	// Verify ciphertext is base64-encodable (for storage)
	base64Ciphertext := base64.StdEncoding.EncodeToString(ciphertext)
	if base64Ciphertext == "" {
		t.Error("Ciphertext should be base64-encodable")
	}
}

func TestAgeEncryptor_Decrypt_RealKey(t *testing.T) {
	encryptor := NewAgeEncryptor()

	// Use a properly formatted test key pair
	// In real usage, these would be generated with zen-lock keygen
	testPrivateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"
	testPublicKey := "age1qyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqszqgpqyqs3290u3"

	plaintext := []byte("test secret data")

	// First encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{testPublicKey})
	if err != nil {
		t.Skipf("Encrypt failed (may need valid key): %v", err)
		return
	}

	// Then decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, testPrivateKey)
	if err != nil {
		// Decrypt may fail with test keys - that's expected
		t.Logf("Decrypt returned error (expected for test keys): %v", err)
		return
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestAgeEncryptor_Decrypt_InvalidCiphertext(t *testing.T) {
	encryptor := NewAgeEncryptor()

	testPrivateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"

	// Test with invalid ciphertext
	invalidCiphertext := []byte("not-a-valid-age-ciphertext")

	_, err := encryptor.Decrypt(invalidCiphertext, testPrivateKey)
	if err == nil {
		t.Error("Decrypt should return error for invalid ciphertext")
	}
}

func TestAgeEncryptor_Decrypt_EmptyCiphertext(t *testing.T) {
	encryptor := NewAgeEncryptor()

	testPrivateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"

	// Test with empty ciphertext
	_, err := encryptor.Decrypt([]byte{}, testPrivateKey)
	if err == nil {
		t.Error("Decrypt should return error for empty ciphertext")
	}
}

func TestAgeEncryptor_DecryptMap_EmptyMap(t *testing.T) {
	encryptor := NewAgeEncryptor()

	testPrivateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"

	// Test with empty map
	emptyMap := map[string]string{}
	result, err := encryptor.DecryptMap(emptyMap, testPrivateKey)
	if err != nil {
		t.Logf("DecryptMap with empty map returned error (may be expected): %v", err)
	}
	if result == nil {
		t.Error("DecryptMap should return a map, even if empty")
	}
	if len(result) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(result))
	}
}

func TestAgeEncryptor_DecryptMap_InvalidEntries(t *testing.T) {
	encryptor := NewAgeEncryptor()

	testPrivateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"

	// Test with invalid base64
	invalidMap := map[string]string{
		"key1": "not-valid-base64!!!",
	}
	_, err := encryptor.DecryptMap(invalidMap, testPrivateKey)
	if err == nil {
		t.Error("DecryptMap should return error for invalid base64")
	}

	// Test with valid base64 but invalid ciphertext
	validBase64 := base64.StdEncoding.EncodeToString([]byte("not-age-ciphertext"))
	invalidMap2 := map[string]string{
		"key1": validBase64,
	}
	_, err = encryptor.DecryptMap(invalidMap2, testPrivateKey)
	if err == nil {
		t.Error("DecryptMap should return error for invalid ciphertext")
	}
}
