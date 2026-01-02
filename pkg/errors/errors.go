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
// This package now uses zen-sdk/pkg/errors as the base implementation.
package errors

import (
	sdkerrors "github.com/kube-zen/zen-sdk/pkg/errors"
)

// ZenLockError is an alias for zen-sdk's ContextError.
// This maintains backward compatibility while using the shared implementation.
type ZenLockError = sdkerrors.ContextError

// WithZenLock adds ZenLock context to an error.
func WithZenLock(err error, namespace, name string) *ZenLockError {
	return sdkerrors.WithMultipleContext(err, map[string]string{
		"zenlock_namespace": namespace,
		"zenlock_name":      name,
	})
}

// WithPod adds Pod context to an error.
func WithPod(err error, namespace, name string) *ZenLockError {
	return sdkerrors.WithMultipleContext(err, map[string]string{
		"pod_namespace": namespace,
		"pod_name":      name,
	})
}

// New creates a new ZenLockError.
func New(errType, message string) *ZenLockError {
	return sdkerrors.New(errType, message)
}

// Wrap wraps an error with a message and type.
func Wrap(err error, errType, message string) *ZenLockError {
	return sdkerrors.Wrap(err, errType, message)
}

// Wrapf wraps an error with a formatted message and type.
func Wrapf(err error, errType, format string, args ...interface{}) *ZenLockError {
	return sdkerrors.Wrapf(err, errType, format, args...)
}
