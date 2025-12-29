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
	"fmt"
	"math"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	// MaxAttempts is the maximum number of retry attempts (default: 3)
	MaxAttempts int
	// InitialDelay is the initial delay before first retry (default: 100ms)
	InitialDelay time.Duration
	// MaxDelay is the maximum delay between retries (default: 5s)
	MaxDelay time.Duration
	// Multiplier is the exponential backoff multiplier (default: 2.0)
	Multiplier float64
	// RetryableErrors is a function that determines if an error is retryable
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		RetryableErrors: func(err error) bool {
			// Retry on transient errors
			if k8serrors.IsServerTimeout(err) || k8serrors.IsTimeout(err) {
				return true
			}
			if k8serrors.IsTooManyRequests(err) {
				return true
			}
			if k8serrors.IsInternalError(err) {
				return true
			}
			// Retry on conflict errors (for optimistic concurrency)
			if k8serrors.IsConflict(err) {
				return true
			}
			return false
		},
	}
}

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.RetryableErrors == nil {
		config.RetryableErrors = DefaultRetryConfig().RetryableErrors
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			return err
		}

		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts-1 {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(float64(delay) * math.Pow(config.Multiplier, float64(attempt)))
			if backoffDelay > config.MaxDelay {
				backoffDelay = config.MaxDelay
			}

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoffDelay):
				// Continue to next attempt
			}
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// RetryWithResult executes a function that returns a result with exponential backoff retry logic
func RetryWithResult[T any](ctx context.Context, config RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}
	if config.RetryableErrors == nil {
		config.RetryableErrors = DefaultRetryConfig().RetryableErrors
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("context cancelled: %w", ctx.Err())
		default:
		}

		// Execute the function
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retryable
		if !config.RetryableErrors(err) {
			return zero, err
		}

		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts-1 {
			// Calculate exponential backoff delay
			backoffDelay := time.Duration(float64(delay) * math.Pow(config.Multiplier, float64(attempt)))
			if backoffDelay > config.MaxDelay {
				backoffDelay = config.MaxDelay
			}

			// Wait with context cancellation support
			select {
			case <-ctx.Done():
				return zero, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(backoffDelay):
				// Continue to next attempt
			}
		}
	}

	return zero, fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

