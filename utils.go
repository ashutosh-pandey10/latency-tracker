// package latencyTracker
package main

import (
	"errors"
	"fmt"
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
		// THIS IS WHERE THE IMPLEMENTATION OF CALCULATION OF LATENCY
		// NEEDS TO BE MADE FOR P99,P95,P50
	}
	return snapshot
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
