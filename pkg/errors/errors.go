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

// Package errors provides structured error types for zen-lock with context.
package errors

import (
	"errors"
	"fmt"
)

// ZenLockError represents a zen-lock error with context.
type ZenLockError struct {
	// Type categorizes the error (e.g., "decryption_failed", "validation_failed", "webhook_failed")
	Type string

	// ZenLockNamespace is the namespace of the ZenLock (if applicable)
	ZenLockNamespace string

	// ZenLockName is the name of the ZenLock (if applicable)
	ZenLockName string

	// PodNamespace is the namespace of the Pod (if applicable)
	PodNamespace string

	// PodName is the name of the Pod (if applicable)
	PodName string

	// Message is the error message
	Message string

	// Err is the underlying error
	Err error
}

// Error implements the error interface.
func (e *ZenLockError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the underlying error.
func (e *ZenLockError) Unwrap() error {
	return e.Err
}

// WithZenLock adds ZenLock context to an error.
func WithZenLock(err error, namespace, name string) *ZenLockError {
	var zlerr *ZenLockError
	if errors.As(err, &zlerr) && zlerr != nil {
		zlerr.ZenLockNamespace = namespace
		zlerr.ZenLockName = name
		return zlerr
	}
	return &ZenLockError{
		Message:          err.Error(),
		Err:              err,
		ZenLockNamespace: namespace,
		ZenLockName:      name,
	}
}

// WithPod adds Pod context to an error.
func WithPod(err error, namespace, name string) *ZenLockError {
	var zlerr *ZenLockError
	if errors.As(err, &zlerr) && zlerr != nil {
		zlerr.PodNamespace = namespace
		zlerr.PodName = name
		return zlerr
	}
	return &ZenLockError{
		Message:      err.Error(),
		Err:          err,
		PodNamespace: namespace,
		PodName:      name,
	}
}

// New creates a new ZenLockError.
func New(errType, message string) *ZenLockError {
	return &ZenLockError{
		Type:    errType,
		Message: message,
	}
}

// Wrap wraps an error with a message and type.
func Wrap(err error, errType, message string) *ZenLockError {
	return &ZenLockError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

// Wrapf wraps an error with a formatted message and type.
func Wrapf(err error, errType, format string, args ...interface{}) *ZenLockError {
	return &ZenLockError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}

