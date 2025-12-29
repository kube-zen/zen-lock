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
	"errors"
	"testing"
	"time"

	zlerrors "github.com/kube-zen/zen-lock/pkg/errors"
)

func TestLogger_logFields(t *testing.T) {
	logger := NewLogger().WithField("key1", "value1").WithField("key2", "value2")
	fields := logger.logFields()

	// logFields should return a flat slice of key-value pairs
	if len(fields) < 4 {
		t.Errorf("Expected at least 4 fields, got %d", len(fields))
	}

	// Verify fields are present (order may vary)
	fieldMap := make(map[interface{}]interface{})
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			fieldMap[fields[i]] = fields[i+1]
		}
	}

	if fieldMap["key1"] != "value1" {
		t.Errorf("Expected key1=value1, got %v", fieldMap["key1"])
	}
	if fieldMap["key2"] != "value2" {
		t.Errorf("Expected key2=value2, got %v", fieldMap["key2"])
	}
}

func TestLogger_Debug(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Debug should not panic
	logger.Debug("test debug message")
}

func TestLogger_Debugf(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Debugf should not panic
	logger.Debugf("test debug message: %s", "formatted")
}

func TestLogger_Info(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Info should not panic
	logger.Info("test info message")
}

func TestLogger_Infof(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Infof should not panic
	logger.Infof("test info message: %s", "formatted")
}

func TestLogger_Warn(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Warn should not panic
	logger.Warn("test warning message")
}

func TestLogger_Warnf(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	// Warnf should not panic
	logger.Warnf("test warning message: %s", "formatted")
}

func TestLogger_Error(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	testErr := errors.New("test error")
	// Error should not panic
	logger.Error(testErr, "test error message")
}

func TestLogger_Errorf(t *testing.T) {
	logger := NewLogger().WithField("test", "value")
	testErr := errors.New("test error")
	// Errorf should not panic
	logger.Errorf(testErr, "test error message: %s", "formatted")
}

func TestLogger_WithCorrelationID(t *testing.T) {
	logger := NewLogger()
	logger = logger.WithCorrelationID("test-correlation-id")

	if logger.fields["correlation_id"] != "test-correlation-id" {
		t.Errorf("Expected correlation_id to be 'test-correlation-id', got %v", logger.fields["correlation_id"])
	}
}

func TestLogger_WithTiming(t *testing.T) {
	logger := NewLogger()
	duration := 250 * time.Millisecond
	logger = logger.WithTiming(duration)

	if logger.fields["duration_ms"] != int64(250) {
		t.Errorf("Expected duration_ms to be 250, got %v", logger.fields["duration_ms"])
	}
}

func TestLogger_WithError_WithZenLockError(t *testing.T) {
	logger := NewLogger()
	zlerr := zlerrors.New("decryption_failed", "decryption failed")
	zlerr.ZenLockNamespace = "test-ns"
	zlerr.ZenLockName = "test-name"
	zlerr.PodNamespace = "pod-ns"
	zlerr.PodName = "pod-name"

	loggerWithErr := logger.WithError(zlerr)

	if loggerWithErr.fields["error_type"] != "decryption_failed" {
		t.Errorf("Expected error_type to be 'decryption_failed', got %v", loggerWithErr.fields["error_type"])
	}
	if loggerWithErr.fields["zenlock_namespace"] != "test-ns" {
		t.Errorf("Expected zenlock_namespace to be 'test-ns', got %v", loggerWithErr.fields["zenlock_namespace"])
	}
	if loggerWithErr.fields["zenlock_name"] != "test-name" {
		t.Errorf("Expected zenlock_name to be 'test-name', got %v", loggerWithErr.fields["zenlock_name"])
	}
	if loggerWithErr.fields["pod_namespace"] != "pod-ns" {
		t.Errorf("Expected pod_namespace to be 'pod-ns', got %v", loggerWithErr.fields["pod_namespace"])
	}
	if loggerWithErr.fields["pod_name"] != "pod-name" {
		t.Errorf("Expected pod_name to be 'pod-name', got %v", loggerWithErr.fields["pod_name"])
	}
}

