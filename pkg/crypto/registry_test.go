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

func TestRegistry_DefaultAlgorithm(t *testing.T) {
	registry := GetGlobalRegistry()
	
	defaultAlg := registry.GetDefault()
	if defaultAlg != "age" {
		t.Errorf("expected default algorithm to be 'age', got %q", defaultAlg)
	}
}

func TestRegistry_IsSupported(t *testing.T) {
	registry := GetGlobalRegistry()
	
	if !registry.IsSupported("age") {
		t.Error("expected 'age' to be supported")
	}
	
	if registry.IsSupported("nonexistent") {
		t.Error("expected 'nonexistent' to not be supported")
	}
}

func TestRegistry_Create(t *testing.T) {
	registry := GetGlobalRegistry()
	
	// Test creating with explicit algorithm
	encryptor, err := registry.Create("age")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	if encryptor == nil {
		t.Fatal("encryptor should not be nil")
	}
	
	// Test creating with empty algorithm (should use default)
	encryptor2, err := registry.Create("")
	if err != nil {
		t.Fatalf("failed to create encryptor with default: %v", err)
	}
	if encryptor2 == nil {
		t.Fatal("encryptor should not be nil")
	}
	
	// Test creating with unsupported algorithm
	_, err = registry.Create("unsupported")
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}

func TestRegistry_GetSupported(t *testing.T) {
	registry := GetGlobalRegistry()
	
	supported := registry.GetSupported()
	if len(supported) == 0 {
		t.Error("expected at least one supported algorithm")
	}
	
	found := false
	for _, alg := range supported {
		if alg == "age" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'age' to be in supported algorithms")
	}
}

func TestCreateEncryptor(t *testing.T) {
	// Test global function
	encryptor, err := CreateEncryptor("age")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	if encryptor == nil {
		t.Fatal("encryptor should not be nil")
	}
	
	// Test with default
	encryptor2, err := CreateEncryptor("")
	if err != nil {
		t.Fatalf("failed to create encryptor with default: %v", err)
	}
	if encryptor2 == nil {
		t.Fatal("encryptor should not be nil")
	}
}

func TestIsAlgorithmSupported(t *testing.T) {
	if !IsAlgorithmSupported("age") {
		t.Error("expected 'age' to be supported")
	}
	
	if IsAlgorithmSupported("unsupported") {
		t.Error("expected 'unsupported' to not be supported")
	}
}

func TestGetSupportedAlgorithms(t *testing.T) {
	algorithms := GetSupportedAlgorithms()
	if len(algorithms) == 0 {
		t.Error("expected at least one supported algorithm")
	}
	
	found := false
	for _, alg := range algorithms {
		if alg == "age" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'age' to be in supported algorithms")
	}
}

func TestRegistry_CustomRegistry(t *testing.T) {
	// Test creating a custom registry
	registry := NewRegistry()
	
	// Register a test algorithm
	testFactory := func() Encryptor {
		return NewAgeEncryptor() // Using age as test, but could be different
	}
	registry.Register("test-alg", testFactory)
	
	if !registry.IsSupported("test-alg") {
		t.Error("expected 'test-alg' to be supported in custom registry")
	}
	
	encryptor, err := registry.Create("test-alg")
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}
	if encryptor == nil {
		t.Fatal("encryptor should not be nil")
	}
}
