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
	"net/http"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// RateLimiter implements token bucket rate limiting for webhook requests
type RateLimiter struct {
	mu          sync.Mutex
	tokens      map[string]*tokenBucket
	maxTokens   int
	refillRate  time.Duration
	cleanupTick *time.Ticker
	stopCh      chan struct{}
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
}

// NewRateLimiter creates a new rate limiter
// maxTokens: maximum number of tokens per key
// refillInterval: time between token refills
func NewRateLimiter(maxTokens int, refillInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		tokens:     make(map[string]*tokenBucket),
		maxTokens:  maxTokens,
		refillRate:  refillInterval,
		stopCh:     make(chan struct{}),
	}

	// Cleanup old entries periodically
	rl.cleanupTick = time.NewTicker(1 * time.Hour)
	go rl.cleanup()

	return rl
}

// Allow checks if a request from the given key should be allowed
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.tokens[key]
	now := time.Now()

	if !exists {
		bucket = &tokenBucket{
			tokens:     rl.maxTokens - 1,
			lastRefill: now,
		}
		rl.tokens[key] = bucket
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(bucket.lastRefill)
	if elapsed >= rl.refillRate {
		refills := int(elapsed / rl.refillRate)
		bucket.tokens = min(rl.maxTokens, bucket.tokens+refills)
		bucket.lastRefill = now
	}

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// cleanup removes old entries to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.cleanupTick.C:
			rl.mu.Lock()
			now := time.Now()
			for key, bucket := range rl.tokens {
				if now.Sub(bucket.lastRefill) > 24*time.Hour {
					delete(rl.tokens, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	select {
	case <-rl.stopCh:
		// Already stopped
	default:
		close(rl.stopCh)
		rl.cleanupTick.Stop()
	}
}

// getClientIP extracts the client IP from the request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies/load balancers)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, take the first one
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (nginx proxy)
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return strings.TrimSpace(realIP)
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RateLimitMiddleware wraps an admission handler with rate limiting
// For admission webhooks, we rate limit based on the source IP
func (rl *RateLimiter) RateLimitMiddleware(next func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := getClientIP(r)
		if !rl.Allow(key) {
			klog.V(2).InfoS("Rate limit exceeded", "client_ip", key, "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded","code":"RATE_LIMITED"}`))
			return
		}
		next(w, r)
	}
}

