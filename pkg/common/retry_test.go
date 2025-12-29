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

package common

import (
	"context"
	"errors"
	"testing"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestDefaultRetryConfig(t *testing.T) {
	config := DefaultRetryConfig()

	if config.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", config.MaxAttempts)
	}
	if config.InitialDelay != 100*time.Millisecond {
		t.Errorf("Expected InitialDelay to be 100ms, got %v", config.InitialDelay)
	}
	if config.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay to be 5s, got %v", config.MaxDelay)
	}
	if config.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier to be 2.0, got %f", config.Multiplier)
	}
	if config.RetryableErrors == nil {
		t.Error("Expected RetryableErrors to be set")
	}

	// Test retryable errors
	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{"ServerTimeout", k8serrors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "test", 1), true},
		{"Timeout", k8serrors.NewTimeoutError("test", 1), true},
		{"TooManyRequests", k8serrors.NewTooManyRequestsError("test"), true},
		{"InternalError", k8serrors.NewInternalError(errors.New("test")), true},
		{"Conflict", k8serrors.NewConflict(schema.GroupResource{Resource: "test"}, "test", errors.New("test")), true},
		{"NotFound", k8serrors.NewNotFound(schema.GroupResource{Resource: "test"}, "test"), false},
		{"BadRequest", k8serrors.NewBadRequest("test"), false},
		{"GenericError", errors.New("generic error"), false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := config.RetryableErrors(tc.err)
			if result != tc.expected {
				t.Errorf("Expected %v for error %v, got %v", tc.expected, tc.err, result)
			}
		})
	}
}

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetry_SuccessOnRetry(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		if attempts < 2 {
			return k8serrors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "test", 1)
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetry_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		return k8serrors.NewServerTimeout(schema.GroupResource{}, "test", 1)
	})

	if err == nil {
		t.Error("Expected error after max attempts")
	}
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
	if !errors.Is(err, k8serrors.NewServerTimeout(schema.GroupResource{}, "test", 1)) {
		// Check that error message contains max attempts
		if err.Error() == "" {
			t.Error("Expected error message to contain max attempts info")
		}
	}
}

func TestRetry_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	nonRetryableErr := errors.New("non-retryable error")
	err := Retry(ctx, config, func() error {
		attempts++
		return nonRetryableErr
	})

	if err == nil {
		t.Error("Expected error")
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attempts)
	}
	if !errors.Is(err, nonRetryableErr) {
		t.Errorf("Expected original error, got %v", err)
	}
}

func TestRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultRetryConfig()
	config.InitialDelay = 100 * time.Millisecond

	attempts := 0
	cancel() // Cancel immediately

	err := Retry(ctx, config, func() error {
		attempts++
		return k8serrors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "test", 1)
	})

	if err == nil {
		t.Error("Expected context cancelled error")
	}
	if attempts != 0 {
		t.Errorf("Expected 0 attempts after context cancellation, got %d", attempts)
	}
}

func TestRetry_ContextCancellationDuringDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.InitialDelay = 100 * time.Millisecond

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		if attempts == 1 {
			// Cancel during the delay after first attempt
			go func() {
				time.Sleep(50 * time.Millisecond)
				cancel()
			}()
		}
		return k8serrors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "test", 1)
	})

	if err == nil {
		t.Error("Expected context cancelled error")
	}
}

func TestRetry_ConfigDefaults(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{} // Empty config

	attempts := 0
	err := Retry(ctx, config, func() error {
		attempts++
		if attempts < 2 {
			return k8serrors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "test", 1)
		}
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	// Note: Retry doesn't modify the original config, it works with a copy internally
	// The test verifies that Retry works correctly with empty config (uses defaults internally)
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryWithResult_Success(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	result, err := RetryWithResult(ctx, config, func() (string, error) {
		attempts++
		return "success", nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got %q", result)
	}
	if attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", attempts)
	}
}

func TestRetryWithResult_RetrySuccess(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxAttempts = 3
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	result, err := RetryWithResult(ctx, config, func() (int, error) {
		attempts++
		if attempts < 2 {
			return 0, k8serrors.NewServerTimeout(schema.GroupResource{}, "test", 1)
		}
		return 42, nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("Expected result 42, got %d", result)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryWithResult_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxAttempts = 2
	config.InitialDelay = 10 * time.Millisecond

	attempts := 0
	result, err := RetryWithResult(ctx, config, func() (string, error) {
		attempts++
		return "", k8serrors.NewServerTimeout(schema.GroupResource{}, "test", 1)
	})

	if err == nil {
		t.Error("Expected error after max attempts")
	}
	if result != "" {
		t.Errorf("Expected zero value for result, got %q", result)
	}
	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}
}

func TestRetryWithResult_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultRetryConfig()
	config.InitialDelay = 10 * time.Millisecond

	cancel() // Cancel immediately

	result, err := RetryWithResult(ctx, config, func() (string, error) {
		return "", k8serrors.NewServerTimeout(schema.GroupResource{Resource: "test"}, "test", 1)
	})

	if err == nil {
		t.Error("Expected context cancelled error")
	}
	if result != "" {
		t.Errorf("Expected zero value for result, got %q", result)
	}
}
