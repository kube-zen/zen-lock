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
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 1*time.Minute)

	if rl == nil {
		t.Fatal("Expected RateLimiter to be created")
	}
	if rl.maxTokens != 10 {
		t.Errorf("Expected maxTokens to be 10, got %d", rl.maxTokens)
	}
	if rl.refillRate != 1*time.Minute {
		t.Errorf("Expected refillRate to be 1 minute, got %v", rl.refillRate)
	}
	if rl.tokens == nil {
		t.Error("Expected tokens map to be initialized")
	}
	if rl.cleanupTick == nil {
		t.Error("Expected cleanupTick to be initialized")
	}

	// Cleanup
	rl.Stop()
}

func TestRateLimiter_Allow_NewKey(t *testing.T) {
	rl := NewRateLimiter(5, 1*time.Minute)
	defer rl.Stop()

	// First request should be allowed
	if !rl.Allow("192.168.1.1") {
		t.Error("Expected first request to be allowed")
	}

	// Should have tokens remaining
	rl.mu.Lock()
	bucket := rl.tokens["192.168.1.1"]
	rl.mu.Unlock()

	if bucket == nil {
		t.Fatal("Expected bucket to be created")
	}
	if bucket.tokens != 4 { // maxTokens - 1
		t.Errorf("Expected 4 tokens remaining, got %d", bucket.tokens)
	}
}

func TestRateLimiter_Allow_Exhausted(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	defer rl.Stop()

	key := "192.168.1.1"

	// First request should be allowed
	if !rl.Allow(key) {
		t.Error("Expected first request to be allowed")
	}

	// Second request should be allowed
	if !rl.Allow(key) {
		t.Error("Expected second request to be allowed")
	}

	// Third request should be denied
	if rl.Allow(key) {
		t.Error("Expected third request to be denied")
	}
}

func TestRateLimiter_Allow_Refill(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)
	defer rl.Stop()

	key := "192.168.1.1"

	// Exhaust tokens
	rl.Allow(key)
	rl.Allow(key)

	// Should be denied
	if rl.Allow(key) {
		t.Error("Expected request to be denied after exhaustion")
	}

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again after refill
	if !rl.Allow(key) {
		t.Error("Expected request to be allowed after refill")
	}
}

func TestRateLimiter_Allow_MultipleKeys(t *testing.T) {
	rl := NewRateLimiter(2, 1*time.Minute)
	defer rl.Stop()

	key1 := "192.168.1.1"
	key2 := "192.168.1.2"

	// Exhaust key1
	rl.Allow(key1)
	rl.Allow(key1)

	// key2 should still work
	if !rl.Allow(key2) {
		t.Error("Expected key2 to be allowed")
	}

	// key1 should be denied
	if rl.Allow(key1) {
		t.Error("Expected key1 to be denied")
	}
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := NewRateLimiter(10, 1*time.Minute)

	// Stop should not panic
	rl.Stop()

	// Calling Stop again should be safe
	rl.Stop()
}

func TestGetClientIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")

	ip := getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected IP 192.168.1.1, got %q", ip)
	}
}

func TestGetClientIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.2")

	ip := getClientIP(req)
	if ip != "192.168.1.2" {
		t.Errorf("Expected IP 192.168.1.2, got %q", ip)
	}
}

func TestGetClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.3:12345"

	ip := getClientIP(req)
	if ip != "192.168.1.3" {
		t.Errorf("Expected IP 192.168.1.3, got %q", ip)
	}
}

func TestGetClientIP_Priority(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.1")
	req.Header.Set("X-Real-IP", "192.168.1.2")
	req.RemoteAddr = "192.168.1.3:12345"

	// X-Forwarded-For should take priority
	ip := getClientIP(req)
	if ip != "192.168.1.1" {
		t.Errorf("Expected X-Forwarded-For IP 192.168.1.1, got %q", ip)
	}
}

func TestMin(t *testing.T) {
	testCases := []struct {
		a, b, expected int
	}{
		{1, 2, 1},
		{2, 1, 1},
		{5, 5, 5},
		{-1, 0, -1},
		{0, -1, -1},
	}

	for _, tc := range testCases {
		result := min(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("min(%d, %d) = %d, expected %d", tc.a, tc.b, result, tc.expected)
		}
	}
}

func TestRateLimitMiddleware_Allowed(t *testing.T) {
	rl := NewRateLimiter(10, 1*time.Minute)
	defer rl.Stop()

	called := false
	next := func(w http.ResponseWriter, r *http.Request) {
		called = true
	}

	handler := rl.RateLimitMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("Expected next handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_RateLimited(t *testing.T) {
	rl := NewRateLimiter(1, 1*time.Minute)
	defer rl.Stop()

	callCount := 0
	next := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}

	handler := rl.RateLimitMiddleware(next)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// First request should be allowed
	w1 := httptest.NewRecorder()
	handler(w1, req)
	if w1.Code != http.StatusOK {
		t.Errorf("Expected first request to be allowed, got status %d", w1.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected next handler to be called once, got %d", callCount)
	}

	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	handler(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w2.Code)
	}
	if callCount != 1 {
		t.Errorf("Expected next handler to still be called once (not called on rate limit), got %d", callCount)
	}
	if w2.Header().Get("Retry-After") != "60" {
		t.Errorf("Expected Retry-After header, got %q", w2.Header().Get("Retry-After"))
	}
}
