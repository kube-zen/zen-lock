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

package logging

import (
	"context"
	"testing"
	"time"

	zlerrors "github.com/kube-zen/zen-lock/pkg/errors"
)

// TestLogger_logFields_Coverage tests logFields to ensure it's covered
// This function is called by all logging methods, so we test it directly
func TestLogger_logFields_Coverage(t *testing.T) {
	logger := NewLogger().
		WithField("key1", "value1").
		WithField("key2", 123).
		WithField("key3", true)

	fields := logger.logFields()

	// logFields should return a flat slice of key-value pairs
	if len(fields) < 6 {
		t.Errorf("Expected at least 6 fields (3 keys + 3 values), got %d", len(fields))
	}

	// Verify fields are interleaved as key-value pairs
	found := make(map[interface{}]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key := fields[i]
			value := fields[i+1]
			found[key] = value
		}
	}

	if found["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %v", found["key1"])
	}
	if found["key2"] != 123 {
		t.Errorf("Expected key2=123, got %v", found["key2"])
	}
	if found["key3"] != true {
		t.Errorf("Expected key3=true, got %v", found["key3"])
	}
}

// TestLogger_logFields_Empty tests logFields with empty logger
func TestLogger_logFields_Empty(t *testing.T) {
	logger := NewLogger()
	fields := logger.logFields()

	if len(fields) != 0 {
		t.Errorf("Expected empty fields for new logger, got %d fields", len(fields))
	}
}

// TestLogger_WithTiming_Coverage tests WithTiming method for coverage
func TestLogger_WithTiming_Coverage(t *testing.T) {
	logger := NewLogger()
	duration := 1500 * time.Millisecond
	loggerWithTiming := logger.WithTiming(duration)

	fields := loggerWithTiming.logFields()
	found := make(map[interface{}]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			found[fields[i]] = fields[i+1]
		}
	}

	if found["duration_ms"] != int64(1500) {
		t.Errorf("Expected duration_ms=1500, got %v", found["duration_ms"])
	}
}

// TestLogger_WithError_NilError tests WithError with nil error
func TestLogger_WithError_NilError(t *testing.T) {
	logger := NewLogger()
	loggerWithErr := logger.WithError(nil)

	// Should return the same logger (no modification)
	if loggerWithErr != logger {
		t.Error("WithError(nil) should return the same logger instance")
	}
}

// TestLogger_WithError_RegularError tests WithError with regular error
func TestLogger_WithError_RegularError(t *testing.T) {
	logger := NewLogger()
	err := zlerrors.New("test_error", "test message")
	loggerWithErr := logger.WithError(err)

	fields := loggerWithErr.logFields()
	found := make(map[interface{}]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			found[fields[i]] = fields[i+1]
		}
	}

	if found["error_type"] != "test_error" {
		t.Errorf("Expected error_type=test_error, got %v", found["error_type"])
	}
	if found["error"] == nil {
		t.Error("Expected error field to be set")
	}
}

// TestLogger_WithError_WithZenLockContext tests WithError with ZenLockError that has context
func TestLogger_WithError_WithZenLockContext(t *testing.T) {
	logger := NewLogger()
	zlerr := zlerrors.New("decryption_failed", "failed to decrypt")
	zlerr.ZenLockNamespace = "test-ns"
	zlerr.ZenLockName = "test-zl"
	zlerr.PodNamespace = "pod-ns"
	zlerr.PodName = "pod-name"

	loggerWithErr := logger.WithError(zlerr)

	fields := loggerWithErr.logFields()
	found := make(map[interface{}]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			found[fields[i]] = fields[i+1]
		}
	}

	if found["error_type"] != "decryption_failed" {
		t.Errorf("Expected error_type=decryption_failed, got %v", found["error_type"])
	}
	if found["zenlock_namespace"] != "test-ns" {
		t.Errorf("Expected zenlock_namespace=test-ns, got %v", found["zenlock_namespace"])
	}
	if found["zenlock_name"] != "test-zl" {
		t.Errorf("Expected zenlock_name=test-zl, got %v", found["zenlock_name"])
	}
	if found["pod_namespace"] != "pod-ns" {
		t.Errorf("Expected pod_namespace=pod-ns, got %v", found["pod_namespace"])
	}
	if found["pod_name"] != "pod-name" {
		t.Errorf("Expected pod_name=pod-name, got %v", found["pod_name"])
	}
}

// TestLogger_AllLoggingMethods tests all logging methods to ensure logFields is called
func TestLogger_AllLoggingMethods(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	err := zlerrors.New("test_error", "test message")

	// These should not panic and should call logFields internally
	logger.Debug("debug message")
	logger.Debugf("debug message: %s", "formatted")
	logger.Info("info message")
	logger.Infof("info message: %s", "formatted")
	logger.Warn("warn message")
	logger.Warnf("warn message: %s", "formatted")
	logger.Error(err, "error message")
	logger.Errorf(err, "error message: %s", "formatted")
}

// TestFromContext_NoLogger tests FromContext when no logger in context
func TestFromContext_NoLogger(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)

	if logger == nil {
		t.Fatal("FromContext should return a logger even if none in context")
	}
}

// TestWithContext_Coverage tests WithContext and FromContext together for coverage
func TestWithContext_Coverage(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	ctx := WithContext(context.Background(), logger)

	retrievedLogger := FromContext(ctx)
	if retrievedLogger == nil {
		t.Fatal("FromContext should return the logger from context")
	}

	// Verify it's the same instance
	if retrievedLogger != logger {
		t.Error("FromContext should return the same logger instance")
	}
}

