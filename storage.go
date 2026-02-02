// package latencyTracker
package main

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Takes Metric receiver, adds new latency value to the slice of
// LatencySamples. Makes sure only MaxSamples number of latencies
// are considered for a given ID/API invocation
func (m *Metric) RecordLatency(latency time.Duration) bool {
	if latency <= 0 {
		fmt.Println("Value passed for latency can't be <= '0'")
		return false
	}
	sample := LatencySample{latency, time.Now()}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.Samples = append(m.Samples, sample)
	if len(m.Samples) > m.Config.MaxSamples {
		// Ensuring that only "MaxSamples" latency records are kept
		m.Samples = m.Samples[1:]
	}
	return true
}

func (m *Metric) CalculateLatency() MetricSnapshot {
	// Considering only those latency samples, that are inside the configured
	// window size, rest will be ignored
	var active []LatencySample
	cutoff := time.Now().Add(-m.Config.Window)
	// WHY IS THIS WINDOWING BEING IMPLEMENTED AT READ-TIME AND NOT WRITE TIME?
	//
	// IDEA : Writes store facts. Reads apply interpretation.
	// Fact: “This latency happened at time T”
	// Interpretation: “Is this within the last 60 seconds?”
	// You store facts.
	// You interpret them at read time.
	// That’s the clean separation.
	//
	// Latency samples are timestamped at ingestion.
	// Sliding windows are enforced at read time because windowing is a query concern,
	// not an ingestion concern.
	// This keeps writes fast, avoids complex cleanup logic, and ensures percentiles
	// always reflect current system behavior.
	m.mu.Lock()
	for _, record := range m.Samples {
		if record.RecordedAt.After(cutoff) {
			active = append(active, record)
		}
	}
	// Instead of defer, used the unlock() directly
	// to free up the mutex earlier, this minimizes the scope of
	// mutex, decreasing the latency of overall operation
	m.mu.Unlock()

	snapshot := MetricSnapshot{
		MetricID: m.Config.ID,
		Window:   m.Config.Window,
		Count:    len(active),
	}
	if len(active) > 0 {
		latencies := make([]time.Duration, 0)
		for _, l := range active {
			latencies = append(latencies, l.LatencyVal)
		}
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		snapshot.P50 = computePercentile(latencies, 50)
		snapshot.P95 = computePercentile(latencies, 95)
		snapshot.P99 = computePercentile(latencies, 99)
	}
	// Since LatencySample has time.Duration as data type, it cannot be directly compared
	return snapshot
}

func computePercentile(latencies []time.Duration, percentile int) *time.Duration {
	N := len(latencies)
	if N == 0 {
		return nil
	}
	idxPercentile := int(math.Ceil(((float64(percentile) / 100.0) * float64(N)))) - 1
	if idxPercentile < 0 {
		idxPercentile = 0
	}
	if idxPercentile >= N {
		idxPercentile = N - 1
	}
	val := latencies[idxPercentile]
	return &val
}

// If you tried to do this: var m map[string]*Metric (without make)
// The map would be nil. If you then tried to assign a value to it
// (m["test"] = &Metric{}), your program would panic. Slices are more
// forgiving; you can append to a nil slice just fine, but maps must
// be initialized with make before you can write to them.
func createNewMetricStore() *MetricStore {
	return &MetricStore{
		Metrics: make(map[string]*Metric),
		mu:      sync.RWMutex{},
	}
}

func (ms *MetricStore) CreateMetric(config MetricConfig) (*Metric, error) {
	if config.Window <= 0 {
		return nil, errors.New("'Window' can't be less than 1!")
	}
	if config.MaxSamples <= 0 {
		return nil, errors.New("'MaxSamples' can't be less than 1!")
	}
	metric := &Metric{
		Config:  config,
		Samples: make([]LatencySample, 0),
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.Metrics[config.ID] = metric
	return metric, nil
}

func (ms *MetricStore) getMetric(id string) (*Metric, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	metric, ok := ms.Metrics[id]
	return metric, ok
}

func (ms *MetricStore) getOrCreateMetric(id string) *Metric {
	// Here we're not using RLock since, it specifically means that
	// no goroutine will write to a shared object, only reads will happend
	// which isn't the case here is it? Refer later part of function!
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if metric, ok := ms.Metrics[id]; ok {
		return metric
	}
	metric := &Metric{
		Config:  defaultMetricConfig(id),
		Samples: make([]LatencySample, 0),
	}

	ms.Metrics = make(map[string]*Metric)
	ms.Metrics[id] = metric
	return metric
}
