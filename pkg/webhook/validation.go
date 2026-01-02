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
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	sdkvalidation "github.com/kube-zen/zen-sdk/pkg/k8s/validation"
)

const (
	// MaxMountPathLength is the maximum reasonable mount path length
	MaxMountPathLength = 1024
)

// ValidateInjectAnnotation validates the zen-lock/inject annotation value
func ValidateInjectAnnotation(injectName string) error {
	// Use zen-sdk validation for Kubernetes resource name validation
	if err := sdkvalidation.ValidateResourceName(injectName, sdkvalidation.MaxAnnotationValueLength); err != nil {
		return fmt.Errorf("inject annotation value: %w", err)
	}

	return nil
}

// ValidateMountPath validates the zen-lock/mount-path annotation value
func ValidateMountPath(mountPath string) error {
	if mountPath == "" {
		return fmt.Errorf("mount path cannot be empty")
	}

	if len(mountPath) > MaxMountPathLength {
		return fmt.Errorf("mount path exceeds maximum length of %d", MaxMountPathLength)
	}

	// Must be an absolute path
	if !filepath.IsAbs(mountPath) {
		return fmt.Errorf("mount path must be an absolute path")
	}

	// Sanitize: prevent directory traversal attempts
	cleanPath := filepath.Clean(mountPath)
	if cleanPath != mountPath {
		return fmt.Errorf("mount path contains invalid characters or directory traversal")
	}

	// Prevent dangerous paths
	dangerousPaths := []string{"/", "/bin", "/sbin", "/usr", "/etc", "/var", "/sys", "/proc", "/dev"}
	for _, dangerous := range dangerousPaths {
		if mountPath == dangerous || strings.HasPrefix(mountPath, dangerous+"/") {
			return fmt.Errorf("mount path cannot be in system directories")
		}
	}

	return nil
}

// SanitizeError sanitizes error messages to prevent information leakage
// Returns a safe error message that doesn't expose sensitive details
func SanitizeError(err error, operation string) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// Remove potential sensitive information patterns
	// Remove full paths
	errMsg = regexp.MustCompile(`/[^\s]+`).ReplaceAllString(errMsg, "[path]")

	// Remove potential secret values (long base64-like strings)
	errMsg = regexp.MustCompile(`[A-Za-z0-9+/]{40,}`).ReplaceAllString(errMsg, "[secret]")

	// Remove IP addresses
	errMsg = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`).ReplaceAllString(errMsg, "[ip]")

	// Generic error message
	return fmt.Errorf("%s failed: %s", operation, errMsg)
}
