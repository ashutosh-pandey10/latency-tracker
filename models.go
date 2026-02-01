// package latencyTracker
package main

import (
	"sync"
	"time"
)

type MetricConfig struct {
	ID          string
	Window      time.Duration
	Percentiles []int // These are the percentiles for which we'll calculate latency
	MaxSamples  int   // Won't accept more that this number of latency values for calculating
	// percentile
	CreatedAt time.Time
}

type LatencySample struct {
	LatencyVal time.Duration
	RecordedAt time.Time
}

// NOTE: Rule of thumb (important):
// Use sync.Mutex by default
// Use sync.RWMutex only when you are sure reads dominate and are cheap
// Reads being cheap mean the slice/struct we're reading over is rather
// consistent, there isn't alot of appending/re-slicing, and the data stru-
// cture is small in size
type Metric struct {
	Config  MetricConfig
	Samples []LatencySample
	mu      sync.Mutex
}

// It is very import in terms of judging performance that we minimize the
// scope of a mutex, because if we use on global mutex for all the operation
// and concurrently multiple operations are trying to read different Metrics,
// this situation only would cause a lot of latency as the mutex lock is blocking
// and we are using same mutex for different operation. Makes sense, right?
type MetricStore struct {
	Metrics map[string]*Metric // The string in the map is essentially the ID assigned to MetricConfig
	mu      sync.RWMutex
}

type MetricSnapshot struct {
	MetricID string
	Window   time.Duration
	Count    int // Latency records for which the latency percentile was calculated

	// 0ms could be a valid latency, You can’t distinguish: “no data yet” or
	// “real value is zero”. This is ambiguous and dangerous
	P50 *time.Duration // Pointer fields allow “not available yet”
	P95 *time.Duration
	P99 *time.Duration
}
