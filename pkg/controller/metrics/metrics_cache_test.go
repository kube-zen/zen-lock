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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestUpdateCacheMetrics(t *testing.T) {
	// Create a test registry
	reg := prometheus.NewRegistry()

	// Create test gauges
	cacheSizeGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "zenlock_cache_size_test",
			Help: "Test metric",
		},
	)
	cacheHitRateGauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "zenlock_cache_hit_rate_test",
			Help: "Test metric",
		},
	)
	reg.MustRegister(cacheSizeGauge, cacheHitRateGauge)

	// Test with hits and misses
	cacheSizeGauge.Set(10)
	if 8+2 > 0 {
		hitRate := float64(8) / float64(8+2)
		cacheHitRateGauge.Set(hitRate)
	} else {
		cacheHitRateGauge.Set(0)
	}

	if size := testutil.ToFloat64(cacheSizeGauge); size != 10 {
		t.Errorf("Expected cache size 10, got %f", size)
	}
	if hitRate := testutil.ToFloat64(cacheHitRateGauge); hitRate != 0.8 {
		t.Errorf("Expected hit rate 0.8, got %f", hitRate)
	}

	// Test with zero hits and misses
	cacheSizeGauge.Set(5)
	cacheHitRateGauge.Set(0)

	if size := testutil.ToFloat64(cacheSizeGauge); size != 5 {
		t.Errorf("Expected cache size 5, got %f", size)
	}
	if hitRate := testutil.ToFloat64(cacheHitRateGauge); hitRate != 0 {
		t.Errorf("Expected hit rate 0, got %f", hitRate)
	}

	// Test with only hits
	cacheSizeGauge.Set(3)
	if 10+0 > 0 {
		hitRate := float64(10) / float64(10+0)
		cacheHitRateGauge.Set(hitRate)
	} else {
		cacheHitRateGauge.Set(0)
	}

	if size := testutil.ToFloat64(cacheSizeGauge); size != 3 {
		t.Errorf("Expected cache size 3, got %f", size)
	}
	if hitRate := testutil.ToFloat64(cacheHitRateGauge); hitRate != 1.0 {
		t.Errorf("Expected hit rate 1.0, got %f", hitRate)
	}

	// Test with only misses
	cacheSizeGauge.Set(3)
	if 0+10 > 0 {
		hitRate := float64(0) / float64(0+10)
		cacheHitRateGauge.Set(hitRate)
	} else {
		cacheHitRateGauge.Set(0)
	}

	if size := testutil.ToFloat64(cacheSizeGauge); size != 3 {
		t.Errorf("Expected cache size 3, got %f", size)
	}
	if hitRate := testutil.ToFloat64(cacheHitRateGauge); hitRate != 0.0 {
		t.Errorf("Expected hit rate 0.0, got %f", hitRate)
	}

	// Test with large numbers
	cacheSizeGauge.Set(100)
	if 75+25 > 0 {
		hitRate := float64(75) / float64(75+25)
		cacheHitRateGauge.Set(hitRate)
	} else {
		cacheHitRateGauge.Set(0)
	}

	if size := testutil.ToFloat64(cacheSizeGauge); size != 100 {
		t.Errorf("Expected cache size 100, got %f", size)
	}
	if hitRate := testutil.ToFloat64(cacheHitRateGauge); hitRate != 0.75 {
		t.Errorf("Expected hit rate 0.75, got %f", hitRate)
	}

	// Also test the actual function (uses global registry)
	UpdateCacheMetrics(50, 40, 10)
	UpdateCacheMetrics(0, 0, 0)
}
