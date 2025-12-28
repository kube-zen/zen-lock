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

// Package validation provides validation utilities for ZenLock CRDs.
package validation

import (
	"fmt"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.zen.io/v1alpha1"
)

// ValidateZenLock validates a ZenLock CRD.
func ValidateZenLock(zenlock *securityv1alpha1.ZenLock) error {
	if zenlock == nil {
		return fmt.Errorf("zenlock is nil")
	}

	if zenlock.Spec.EncryptedData == nil || len(zenlock.Spec.EncryptedData) == 0 {
		return fmt.Errorf("encryptedData cannot be empty")
	}

	// Validate algorithm
	if zenlock.Spec.Algorithm != "" && zenlock.Spec.Algorithm != "age" {
		return fmt.Errorf("unsupported algorithm: %s (only 'age' is supported)", zenlock.Spec.Algorithm)
	}

	// Validate encrypted data format (should be base64 strings)
	for key, value := range zenlock.Spec.EncryptedData {
		if key == "" {
			return fmt.Errorf("encryptedData key cannot be empty")
		}
		if value == "" {
			return fmt.Errorf("encryptedData value for key %q cannot be empty", key)
		}
		// Note: We don't validate base64 format here as it's expensive
		// The decryption will fail if it's invalid
	}

	// Validate allowed subjects
	for i, subject := range zenlock.Spec.AllowedSubjects {
		if err := ValidateSubjectReference(&subject); err != nil {
			return fmt.Errorf("allowedSubjects[%d]: %w", i, err)
		}
	}

	return nil
}

// ValidateSubjectReference validates a SubjectReference.
func ValidateSubjectReference(subject *securityv1alpha1.SubjectReference) error {
	if subject == nil {
		return fmt.Errorf("subject is nil")
	}

	if subject.Kind == "" {
		return fmt.Errorf("kind is required")
	}

	if subject.Name == "" {
		return fmt.Errorf("name is required")
	}

	// Validate kind
	validKinds := map[string]bool{
		"ServiceAccount": true,
		"User":           true,
		"Group":          true,
	}
	if !validKinds[subject.Kind] {
		return fmt.Errorf("invalid kind: %s (must be ServiceAccount, User, or Group)", subject.Kind)
	}

	// ServiceAccount requires namespace
	if subject.Kind == "ServiceAccount" && subject.Namespace == "" {
		return fmt.Errorf("namespace is required for ServiceAccount kind")
	}

	return nil
}

