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

// Package logging provides structured logging with correlation IDs and consistent field formatting.
package logging

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/klog/v2"

	zlerrors "github.com/kube-zen/zen-lock/pkg/errors"
)

// Logger provides structured logging with consistent fields and correlation IDs.
type Logger struct {
	fields map[string]interface{}
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value interface{}
}

// NewLogger creates a new logger instance.
func NewLogger() *Logger {
	return &Logger{
		fields: make(map[string]interface{}),
	}
}

// WithFields creates a new logger with additional fields.
func (l *Logger) WithFields(fields ...Field) *Logger {
	newLogger := &Logger{
		fields: make(map[string]interface{}),
	}
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Add new fields
	for _, f := range fields {
		newLogger.fields[f.Key] = f.Value
	}
	return newLogger
}

// WithField creates a new logger with a single additional field.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(Field{Key: key, Value: value})
}

// WithZenLock adds ZenLock-related fields to the logger.
func (l *Logger) WithZenLock(namespace, name string) *Logger {
	return l.WithFields(
		Field{Key: "zenlock_namespace", Value: namespace},
		Field{Key: "zenlock_name", Value: name},
	)
}

// WithPod adds Pod-related fields to the logger.
func (l *Logger) WithPod(namespace, name string) *Logger {
	return l.WithFields(
		Field{Key: "pod_namespace", Value: namespace},
		Field{Key: "pod_name", Value: name},
	)
}

// WithCorrelationID adds a correlation ID to the logger for tracing.
func (l *Logger) WithCorrelationID(correlationID string) *Logger {
	return l.WithField("correlation_id", correlationID)
}

// WithError adds error information to the logger.
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}

	var zlerr *zlerrors.ZenLockError
	if errors.As(err, &zlerr) && zlerr != nil {
		logger := l.WithField("error_type", zlerr.Type)
		if zlerr.ZenLockNamespace != "" {
			logger = logger.WithField("zenlock_namespace", zlerr.ZenLockNamespace)
		}
		if zlerr.ZenLockName != "" {
			logger = logger.WithField("zenlock_name", zlerr.ZenLockName)
		}
		if zlerr.PodNamespace != "" {
			logger = logger.WithField("pod_namespace", zlerr.PodNamespace)
		}
		if zlerr.PodName != "" {
			logger = logger.WithField("pod_name", zlerr.PodName)
		}
		return logger.WithField("error", err.Error())
	}

	return l.WithField("error", err.Error())
}

// logFields formats fields for klog.
func (l *Logger) logFields() []interface{} {
	fields := make([]interface{}, 0, len(l.fields)*2)
	for k, v := range l.fields {
		fields = append(fields, k, v)
	}
	return fields
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string) {
	klog.V(4).InfoS(msg, l.logFields()...)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.V(4).InfoS(msg, l.logFields()...)
}

// Info logs an info message.
func (l *Logger) Info(msg string) {
	klog.InfoS(msg, l.logFields()...)
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.InfoS(msg, l.logFields()...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string) {
	klog.InfoS(msg, append([]interface{}{"level", "warning"}, l.logFields()...)...)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.InfoS(msg, append([]interface{}{"level", "warning"}, l.logFields()...)...)
}

// Error logs an error message.
func (l *Logger) Error(err error, msg string) {
	klog.ErrorS(err, msg, l.logFields()...)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(err error, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	klog.ErrorS(err, msg, l.logFields()...)
}

// FromContext extracts a logger from context or creates a new one.
func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*Logger); ok {
		return logger
	}
	return NewLogger()
}

// WithContext adds a logger to context.
func WithContext(ctx context.Context, logger *Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

type loggerKey struct{}

// WithTiming logs the duration of an operation.
func (l *Logger) WithTiming(duration time.Duration) *Logger {
	return l.WithField("duration_ms", duration.Milliseconds())
}
