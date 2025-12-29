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

package webhook

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	securityv1alpha1 "github.com/kube-zen/zen-lock/pkg/apis/security.kube-zen.io/v1alpha1"
	"github.com/kube-zen/zen-lock/pkg/crypto"
)

// ZenLockValidatorHandler is an admission handler that validates ZenLock CRDs
type ZenLockValidatorHandler struct {
	decoder   admission.Decoder
	validator *ZenLockValidator
}

// ZenLockValidator validates ZenLock CRDs
type ZenLockValidator struct {
	crypto     crypto.Encryptor
	privateKey string
}

// NewZenLockValidator creates a new ZenLock validator
func NewZenLockValidator(scheme *runtime.Scheme) (*ZenLockValidator, error) {
	// Load private key from environment for validation
	privateKey := os.Getenv("ZEN_LOCK_PRIVATE_KEY")
	if privateKey == "" {
		return nil, fmt.Errorf("ZEN_LOCK_PRIVATE_KEY environment variable is not set")
	}

	// Initialize crypto
	encryptor := crypto.NewAgeEncryptor()

	return &ZenLockValidator{
		crypto:     encryptor,
		privateKey: privateKey,
	}, nil
}

// NewZenLockValidatorHandler creates a new admission handler for ZenLock validation
func NewZenLockValidatorHandler(scheme *runtime.Scheme) (*ZenLockValidatorHandler, error) {
	decoder := admission.NewDecoder(scheme)
	validator, err := NewZenLockValidator(scheme)
	if err != nil {
		return nil, err
	}

	return &ZenLockValidatorHandler{
		decoder:   decoder,
		validator: validator,
	}, nil
}

// Handle processes admission requests for ZenLock validation
func (h *ZenLockValidatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	zenlock := &securityv1alpha1.ZenLock{}

	if err := h.decoder.Decode(req, zenlock); err != nil {
		return admission.Errored(400, err)
	}

	var err error
	switch req.Operation {
	case admissionv1.Create:
		err = h.validator.validateZenLock(zenlock)
	case admissionv1.Update:
		// Validate the new object (old object decoding is optional)
		_ = h.decoder.DecodeRaw(req.OldObject, &securityv1alpha1.ZenLock{})
		err = h.validator.validateZenLock(zenlock)
	case admissionv1.Delete:
		// Allow deletion - finalizers handle cleanup
		return admission.Allowed("")
	default:
		return admission.Allowed("")
	}

	if err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("")
}

// validateZenLock validates a ZenLock CRD
func (v *ZenLockValidator) validateZenLock(zenlock *securityv1alpha1.ZenLock) error {
	// Validate encrypted data is not empty
	if len(zenlock.Spec.EncryptedData) == 0 {
		return fmt.Errorf("encryptedData cannot be empty")
	}

	// Validate algorithm (if specified)
	if zenlock.Spec.Algorithm != "" && zenlock.Spec.Algorithm != "age" {
		return fmt.Errorf("unsupported algorithm %q, only 'age' is currently supported", zenlock.Spec.Algorithm)
	}

	// Validate encrypted data format (must be valid base64)
	for key, value := range zenlock.Spec.EncryptedData {
		if value == "" {
			return fmt.Errorf("encryptedData[%q] cannot be empty", key)
		}
		if _, err := base64.StdEncoding.DecodeString(value); err != nil {
			return fmt.Errorf("encryptedData[%q] is not valid base64: %v", key, err)
		}
	}

	// Validate AllowedSubjects
	for i, subject := range zenlock.Spec.AllowedSubjects {
		if subject.Kind == "" {
			return fmt.Errorf("allowedSubjects[%d].kind is required", i)
		}
		if subject.Kind != "ServiceAccount" {
			return fmt.Errorf("allowedSubjects[%d].kind must be ServiceAccount (got %q)", i, subject.Kind)
		}
		if subject.Name == "" {
			return fmt.Errorf("allowedSubjects[%d].name is required", i)
		}
		if subject.Namespace == "" {
			return fmt.Errorf("allowedSubjects[%d].namespace is required for ServiceAccount", i)
		}
	}

	// Try to decrypt to verify the data is valid (optional - can be expensive)
	// Only validate if we have a private key
	if v.privateKey != "" {
		_, err := v.crypto.DecryptMap(zenlock.Spec.EncryptedData, v.privateKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt encryptedData: %v (data may be encrypted with a different key)", err)
		}
	}

	return nil
}
