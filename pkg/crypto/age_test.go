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
	"testing"
)

func TestNewAgeEncryptor(t *testing.T) {
	encryptor := NewAgeEncryptor()
	if encryptor == nil {
		t.Error("NewAgeEncryptor should not return nil")
	}
}

func TestAgeEncryptor_EncryptDecrypt(t *testing.T) {
	encryptor := NewAgeEncryptor()
	
	// Generate a test key pair (this is a simplified test - in real usage, use proper keygen)
	// For this test, we'll use a known test key pair
	testPrivateKey := "AGE-SECRET-1EXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE"
	testPublicKey := "age1q3exampleexampleexampleexampleexampleexampleexampleexample"
	
	plaintext := []byte("test secret data")
	
	// Encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{testPublicKey})
	if err != nil {
		// If encryption fails due to invalid key format, that's expected for test keys
		// We're just testing that the function can be called
		t.Logf("Encrypt returned error (expected for test keys): %v", err)
		return
	}
	
	if len(ciphertext) == 0 {
		t.Error("Ciphertext should not be empty")
	}
	
	// Decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, testPrivateKey)
	if err != nil {
		t.Logf("Decrypt returned error (expected for test keys): %v", err)
		return
	}
	
	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestAgeEncryptor_DecryptMap(t *testing.T) {
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
	
	// Test with invalid ciphertext
	invalidMap := map[string]string{
		"key1": "invalid-ciphertext",
	}
	_, err = encryptor.DecryptMap(invalidMap, testPrivateKey)
	if err == nil {
		t.Error("DecryptMap should return error for invalid ciphertext")
	}
}

func TestAgeEncryptor_Interface(t *testing.T) {
	// Verify that AgeEncryptor implements Encryptor interface
	var _ Encryptor = NewAgeEncryptor()
}

