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

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	if logger.fields == nil {
		t.Error("Logger fields map is nil")
	}
}

func TestLogger_WithField(t *testing.T) {
	logger := NewLogger()
	logger = logger.WithField("key", "value")

	if logger.fields["key"] != "value" {
		t.Errorf("Expected field 'key' to be 'value', got %v", logger.fields["key"])
	}
}

func TestLogger_WithFields(t *testing.T) {
	logger := NewLogger()
	logger = logger.WithFields(
		Field{Key: "key1", Value: "value1"},
		Field{Key: "key2", Value: "value2"},
	)

	if logger.fields["key1"] != "value1" {
		t.Errorf("Expected field 'key1' to be 'value1', got %v", logger.fields["key1"])
	}
	if logger.fields["key2"] != "value2" {
		t.Errorf("Expected field 'key2' to be 'value2', got %v", logger.fields["key2"])
	}
}

func TestLogger_WithZenLock(t *testing.T) {
	logger := NewLogger()
	logger = logger.WithZenLock("test-namespace", "test-name")

	if logger.fields["zenlock_namespace"] != "test-namespace" {
		t.Errorf("Expected zenlock_namespace to be 'test-namespace', got %v", logger.fields["zenlock_namespace"])
	}
	if logger.fields["zenlock_name"] != "test-name" {
		t.Errorf("Expected zenlock_name to be 'test-name', got %v", logger.fields["zenlock_name"])
	}
}

func TestLogger_WithPod(t *testing.T) {
	logger := NewLogger()
	logger = logger.WithPod("test-namespace", "test-pod")

	if logger.fields["pod_namespace"] != "test-namespace" {
		t.Errorf("Expected pod_namespace to be 'test-namespace', got %v", logger.fields["pod_namespace"])
	}
	if logger.fields["pod_name"] != "test-pod" {
		t.Errorf("Expected pod_name to be 'test-pod', got %v", logger.fields["pod_name"])
	}
}

func TestLogger_WithCorrelationID(t *testing.T) {
	logger := NewLogger()
	logger = logger.WithCorrelationID("test-correlation-id")

	if logger.fields["correlation_id"] != "test-correlation-id" {
		t.Errorf("Expected correlation_id to be 'test-correlation-id', got %v", logger.fields["correlation_id"])
	}
}

func TestLogger_WithError(t *testing.T) {
	logger := NewLogger()
	regularErr := logger.WithError(nil)
	if regularErr != logger {
		t.Error("WithError(nil) should return the same logger")
	}

	simpleErr := logger.WithError(context.DeadlineExceeded)
	if simpleErr.fields["error"] == nil {
		t.Error("WithError should add error field")
	}

	zlerr := zlerrors.New("decryption_failed", "decryption failed")
	loggerWithErr := logger.WithError(zlerr)
	if loggerWithErr.fields["error_type"] != "decryption_failed" {
		t.Errorf("Expected error_type to be 'decryption_failed', got %v", loggerWithErr.fields["error_type"])
	}
}

func TestLogger_WithTiming(t *testing.T) {
	logger := NewLogger()
	duration := 100 * time.Millisecond
	logger = logger.WithTiming(duration)

	if logger.fields["duration_ms"] != int64(100) {
		t.Errorf("Expected duration_ms to be 100, got %v", logger.fields["duration_ms"])
	}
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("FromContext returned nil")
	}

	// Test with logger in context
	testLogger := NewLogger().WithField("test", "value")
	ctxWithLogger := WithContext(ctx, testLogger)
	retrievedLogger := FromContext(ctxWithLogger)
	if retrievedLogger.fields["test"] != "value" {
		t.Errorf("Expected to retrieve logger from context, got %v", retrievedLogger.fields)
	}
}

func TestWithContext(t *testing.T) {
	ctx := context.Background()
	logger := NewLogger().WithField("test", "value")
	ctxWithLogger := WithContext(ctx, logger)

	retrievedLogger := FromContext(ctxWithLogger)
	if retrievedLogger.fields["test"] != "value" {
		t.Errorf("Expected to retrieve logger from context, got %v", retrievedLogger.fields)
	}
}

func TestLogger_FieldsAreImmutable(t *testing.T) {
	logger1 := NewLogger().WithField("key1", "value1")
	logger2 := logger1.WithField("key2", "value2")

	if logger1.fields["key2"] != nil {
		t.Error("logger1 should not have key2")
	}
	if logger2.fields["key1"] != "value1" {
		t.Error("logger2 should have key1")
	}
	if logger2.fields["key2"] != "value2" {
		t.Error("logger2 should have key2")
	}
}
