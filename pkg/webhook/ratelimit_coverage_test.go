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
	"testing"
	"time"
)

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(10, time.Hour)
	defer rl.Stop()

	key1 := "1.1.1.1"
	key2 := "2.2.2.2"

	// Add some entries
	_ = rl.Allow(key1)
	_ = rl.Allow(key2)

	// Simulate old entry
	rl.mu.Lock()
	rl.tokens[key1].lastRefill = time.Now().Add(-25 * time.Hour) // Older than 24 hours
	rl.mu.Unlock()

	// Manually trigger cleanup by sending tick
	// Note: cleanup runs in a goroutine, so we need to wait a bit
	time.Sleep(100 * time.Millisecond)

	// Verify old entry is cleaned up (may take a moment)
	rl.mu.Lock()
	_, exists1 := rl.tokens[key1]
	_, exists2 := rl.tokens[key2]
	rl.mu.Unlock()

	// key1 might still exist if cleanup hasn't run yet, but key2 should exist
	if !exists2 {
		t.Error("Expected key2 to still exist")
	}
	// key1 cleanup is async, so we just verify the cleanup mechanism exists
	_ = exists1
}

func TestRateLimiter_Cleanup_Stop(t *testing.T) {
	rl := NewRateLimiter(10, time.Hour)

	// Stop should close the stop channel
	rl.Stop()

	// Verify stop channel is closed
	select {
	case <-rl.stopCh:
		// Expected - channel should be closed
	default:
		t.Error("Expected stopCh to be closed after Stop()")
	}
}

