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
	"fmt"
	"sync"
)

// EncryptorFactory is a function that creates a new Encryptor instance
type EncryptorFactory func() Encryptor

// Registry manages encryption algorithm implementations
type Registry struct {
	mu        sync.RWMutex
	factories map[string]EncryptorFactory
	defaultAlg string
}

var (
	// globalRegistry is the default global registry
	globalRegistry *Registry
	initOnce       sync.Once
)

// initRegistry initializes the global registry with default algorithms
func initRegistry() {
	globalRegistry = NewRegistry()
	// Register default algorithm
	globalRegistry.Register("age", func() Encryptor {
		return NewAgeEncryptor()
	})
	globalRegistry.SetDefault("age")
}

// NewRegistry creates a new empty registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]EncryptorFactory),
		defaultAlg: "age",
	}
}

// Register registers an encryption algorithm factory
func (r *Registry) Register(name string, factory EncryptorFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// SetDefault sets the default algorithm name
func (r *Registry) SetDefault(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultAlg = name
}

// GetDefault returns the default algorithm name
func (r *Registry) GetDefault() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultAlg
}

// GetSupported returns a list of supported algorithm names
func (r *Registry) GetSupported() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	algorithms := make([]string, 0, len(r.factories))
	for name := range r.factories {
		algorithms = append(algorithms, name)
	}
	return algorithms
}

// IsSupported checks if an algorithm is supported
func (r *Registry) IsSupported(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[name]
	return exists
}

// Create creates an Encryptor instance for the given algorithm
// If algorithm is empty, uses the default algorithm
func (r *Registry) Create(algorithm string) (Encryptor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use default if algorithm is empty
	if algorithm == "" {
		algorithm = r.defaultAlg
	}

	factory, exists := r.factories[algorithm]
	if !exists {
		return nil, fmt.Errorf("unsupported algorithm: %s (supported: %v)", algorithm, r.getSupportedLocked())
	}

	return factory(), nil
}

// getSupportedLocked returns supported algorithms (must be called with lock held)
func (r *Registry) getSupportedLocked() []string {
	algorithms := make([]string, 0, len(r.factories))
	for name := range r.factories {
		algorithms = append(algorithms, name)
	}
	return algorithms
}

// GetGlobalRegistry returns the global registry instance
func GetGlobalRegistry() *Registry {
	initOnce.Do(initRegistry)
	return globalRegistry
}

// CreateEncryptor creates an Encryptor using the global registry
// This is the main entry point for creating encryptors
func CreateEncryptor(algorithm string) (Encryptor, error) {
	return GetGlobalRegistry().Create(algorithm)
}

// IsAlgorithmSupported checks if an algorithm is supported using the global registry
func IsAlgorithmSupported(algorithm string) bool {
	return GetGlobalRegistry().IsSupported(algorithm)
}

// GetSupportedAlgorithms returns all supported algorithm names
func GetSupportedAlgorithms() []string {
	return GetGlobalRegistry().GetSupported()
}

// GetDefaultAlgorithm returns the default algorithm name
func GetDefaultAlgorithm() string {
	return GetGlobalRegistry().GetDefault()
}
