/*
Copyright 2025 Kube-ZEN Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on "AS IS" BASIS,
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

// generateTestKeyPair generates a new age key pair for testing
func generateTestKeyPair(t *testing.T) (privateKey string, publicKey string) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("Failed to generate test identity: %v", err)
	}
	return identity.String(), identity.Recipient().String()
}

func TestAgeEncryptor_Encrypt_Success(t *testing.T) {
	encryptor := NewAgeEncryptor()
	privateKey, publicKey := generateTestKeyPair(t)

	plaintext := []byte("test secret data")

	// Encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want no error", err)
	}

	if len(ciphertext) == 0 {
		t.Error("Ciphertext should not be empty")
	}

	// Verify we can decrypt it
	decrypted, err := encryptor.Decrypt(ciphertext, privateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v, want no error", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestAgeEncryptor_Encrypt_InvalidRecipient(t *testing.T) {
	encryptor := NewAgeEncryptor()

	plaintext := []byte("test data")
	_, err := encryptor.Encrypt(plaintext, []string{"invalid-public-key"})

	if err == nil {
		t.Error("Encrypt() should return error for invalid recipient")
	}
}

func TestAgeEncryptor_Decrypt_Success(t *testing.T) {
	encryptor := NewAgeEncryptor()
	privateKey, publicKey := generateTestKeyPair(t)

	plaintext := []byte("test secret data")

	// First encrypt
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want no error", err)
	}

	// Then decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, privateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v, want no error", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("Decrypted text doesn't match: got %s, want %s", string(decrypted), string(plaintext))
	}
}

func TestAgeEncryptor_Decrypt_WrongKey(t *testing.T) {
	encryptor := NewAgeEncryptor()
	_, publicKey1 := generateTestKeyPair(t)
	privateKey2, _ := generateTestKeyPair(t)

	plaintext := []byte("test data")
	ciphertext, err := encryptor.Encrypt(plaintext, []string{publicKey1})
	if err != nil {
		t.Fatalf("Encrypt() error = %v, want no error", err)
	}

	// Try to decrypt with wrong private key
	_, err = encryptor.Decrypt(ciphertext, privateKey2)
	if err == nil {
		t.Error("Decrypt() should return error when using wrong private key")
	}
}


func TestAgeEncryptor_DecryptMap_Success(t *testing.T) {
	encryptor := NewAgeEncryptor()
	privateKey, publicKey := generateTestKeyPair(t)

	// Encrypt multiple values
	plaintext1 := []byte("value1")
	plaintext2 := []byte("value2")

	ciphertext1, err := encryptor.Encrypt(plaintext1, []string{publicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	ciphertext2, err := encryptor.Encrypt(plaintext2, []string{publicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encryptedData := map[string]string{
		"key1": base64.StdEncoding.EncodeToString(ciphertext1),
		"key2": base64.StdEncoding.EncodeToString(ciphertext2),
	}

	// Decrypt map
	result, err := encryptor.DecryptMap(encryptedData, privateKey)
	if err != nil {
		t.Fatalf("DecryptMap() error = %v, want no error", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 decrypted values, got %d", len(result))
	}

	if string(result["key1"]) != string(plaintext1) {
		t.Errorf("key1: got %s, want %s", string(result["key1"]), string(plaintext1))
	}

	if string(result["key2"]) != string(plaintext2) {
		t.Errorf("key2: got %s, want %s", string(result["key2"]), string(plaintext2))
	}
}


func TestAgeEncryptor_EncryptDecrypt_RoundTrip(t *testing.T) {
	encryptor := NewAgeEncryptor()
	privateKey, publicKey := generateTestKeyPair(t)

	testCases := []struct {
		name      string
		plaintext []byte
	}{
		{"short text", []byte("hello")},
		{"long text", []byte("this is a much longer piece of text that should also encrypt and decrypt correctly")},
		{"empty", []byte("")},
		{"binary data", []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}},
		{"unicode", []byte("Hello 世界 🌍")},
		{"newlines", []byte("line1\nline2\nline3")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := encryptor.Encrypt(tc.plaintext, []string{publicKey})
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			// Decrypt
			decrypted, err := encryptor.Decrypt(ciphertext, privateKey)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			// Verify
			if string(decrypted) != string(tc.plaintext) {
				t.Errorf("Round-trip failed: got %q, want %q", string(decrypted), string(tc.plaintext))
			}
		})
	}
}

func TestAgeEncryptor_Encrypt_LargeData(t *testing.T) {
	encryptor := NewAgeEncryptor()
	privateKey, publicKey := generateTestKeyPair(t)

	// Create large plaintext (1MB)
	largePlaintext := make([]byte, 1024*1024)
	for i := range largePlaintext {
		largePlaintext[i] = byte(i % 256)
	}

	// Encrypt
	ciphertext, err := encryptor.Encrypt(largePlaintext, []string{publicKey})
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, privateKey)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Verify
	if len(decrypted) != len(largePlaintext) {
		t.Errorf("Decrypted length doesn't match: got %d, want %d", len(decrypted), len(largePlaintext))
	}

	for i := range largePlaintext {
		if decrypted[i] != largePlaintext[i] {
			t.Errorf("Mismatch at index %d: got %d, want %d", i, decrypted[i], largePlaintext[i])
			break
		}
	}
}

func TestAgeEncryptor_DecryptMap_MultipleKeys(t *testing.T) {
	encryptor := NewAgeEncryptor()
	privateKey, publicKey := generateTestKeyPair(t)

	// Encrypt multiple values
	values := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	encryptedData := make(map[string]string)
	for k, v := range values {
		ciphertext, err := encryptor.Encrypt(v, []string{publicKey})
		if err != nil {
			t.Fatalf("Encrypt() error for %s: %v", k, err)
		}
		encryptedData[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	// Decrypt map
	result, err := encryptor.DecryptMap(encryptedData, privateKey)
	if err != nil {
		t.Fatalf("DecryptMap() error = %v", err)
	}

	if len(result) != len(values) {
		t.Errorf("Expected %d decrypted values, got %d", len(values), len(result))
	}

	for k, expected := range values {
		if actual, ok := result[k]; !ok {
			t.Errorf("Missing key %s in result", k)
		} else if string(actual) != string(expected) {
			t.Errorf("Key %s: got %s, want %s", k, string(actual), string(expected))
		}
	}
}

