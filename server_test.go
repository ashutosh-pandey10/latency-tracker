package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Test handleRecordLatency with valid input
func TestHandleRecordLatencyValid(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	body := recordLatencyRequest{LatencyNs: 1000000} // 1ms
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/metrics/test-metric/latency", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	server.handleRecordLatency(w, req, "test-metric")

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}

	// Verify metric was created and latency recorded
	metric, ok := store.getMetric("test-metric")
	if !ok {
		t.Errorf("Expected metric to be created")
	}

	if len(metric.Samples) != 1 {
		t.Errorf("Expected 1 sample, got %d", len(metric.Samples))
	}
}

// Test handleRecordLatency with zero latency
func TestHandleRecordLatencyZeroValue(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	body := recordLatencyRequest{LatencyNs: 0}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/metrics/test-metric/latency", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	server.handleRecordLatency(w, req, "test-metric")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleRecordLatency with negative latency
func TestHandleRecordLatencyNegativeValue(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	body := recordLatencyRequest{LatencyNs: -100}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/metrics/test-metric/latency", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	server.handleRecordLatency(w, req, "test-metric")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleRecordLatency with invalid JSON
func TestHandleRecordLatencyInvalidJSON(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	invalidJSON := bytes.NewReader([]byte("{invalid json}"))

	req := httptest.NewRequest("POST", "/metrics/test-metric/latency", invalidJSON)
	w := httptest.NewRecorder()

	server.handleRecordLatency(w, req, "test-metric")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

// Test handleGetMetric for existing metric
func TestHandleGetMetricExists(t *testing.T) {
	store := createNewMetricStore()
	config := defaultMetricConfig("test-metric")
	store.CreateMetric(config)

	metric, _ := store.getMetric("test-metric")
	metric.RecordLatency(time.Duration(1000000)) // 1ms

	server := newServer(store)

	req := httptest.NewRequest("GET", "/metrics/test-metric", nil)
	w := httptest.NewRecorder()

	server.handleGetMetric(w, req, "test-metric")

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
	}

	var snapshot MetricSnapshot
	err := json.NewDecoder(w.Body).Decode(&snapshot)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if snapshot.MetricID != "test-metric" {
		t.Errorf("Expected metric ID test-metric, got %s", snapshot.MetricID)
	}
}

// Test handleGetMetric for non-existing metric
func TestHandleGetMetricNotFound(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	req := httptest.NewRequest("GET", "/metrics/non-existent", nil)
	w := httptest.NewRecorder()

	server.handleGetMetric(w, req, "non-existent")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Test handleMetrics with multiple metrics
func TestHandleMetricsMultiple(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	// Create multiple metrics
	for i := 1; i <= 3; i++ {
		config := defaultMetricConfig("metric-" + string(rune('0'+i)))
		store.CreateMetric(config)
		metric, _ := store.getMetric(config.ID)
		metric.RecordLatency(time.Duration(1000000 * i))
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var snapshots []MetricSnapshot
	err := json.NewDecoder(w.Body).Decode(&snapshots)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(snapshots) != 3 {
		t.Errorf("Expected 3 snapshots, got %d", len(snapshots))
	}
}

// Test handleMetrics with non-GET request
func TestHandleMetricsNonGET(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	req := httptest.NewRequest("POST", "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// Test handleMetrics with empty store
func TestHandleMetricsEmpty(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	server.handleMetrics(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var snapshots []MetricSnapshot
	err := json.NewDecoder(w.Body).Decode(&snapshots)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("Expected 0 snapshots, got %d", len(snapshots))
	}
}

// Test handleMetricsById with invalid path
func TestHandleMetricsByIdInvalidPath(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	req := httptest.NewRequest("GET", "/metrics/", nil)
	w := httptest.NewRecorder()

	server.handleMetricsById(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

// Test handleMetricsById POST to /metrics/{id}/latency
func TestHandleMetricsByIdRecordLatency(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	body := recordLatencyRequest{LatencyNs: 5000000}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/metrics/api-123/latency", bytes.NewReader(jsonBody))
	w := httptest.NewRecorder()

	server.handleMetricsById(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}
}

// Test handleMetricsById GET to /metrics/{id}
func TestHandleMetricsByIdGetMetric(t *testing.T) {
	store := createNewMetricStore()
	config := defaultMetricConfig("api-123")
	store.CreateMetric(config)
	metric, _ := store.getMetric("api-123")
	metric.RecordLatency(time.Duration(5000000))

	server := newServer(store)

	req := httptest.NewRequest("GET", "/metrics/api-123", nil)
	w := httptest.NewRecorder()

	server.handleMetricsById(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Test concurrent recording to same metric
func TestHandleRecordLatencyConcurrent(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(latency int64) {
			body := recordLatencyRequest{LatencyNs: latency}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/metrics/concurrent-test/latency", bytes.NewReader(jsonBody))
			w := httptest.NewRecorder()

			server.handleRecordLatency(w, req, "concurrent-test")
			done <- true
		}(int64(1000000 * (i + 1)))
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	metric, ok := store.getMetric("concurrent-test")
	if !ok {
		t.Errorf("Expected metric to exist")
	}

	if len(metric.Samples) != numGoroutines {
		t.Errorf("Expected %d samples, got %d", numGoroutines, len(metric.Samples))
	}
}

// Test newServer constructor
func TestNewServer(t *testing.T) {
	store := createNewMetricStore()
	server := newServer(store)

	if server.store != store {
		t.Errorf("Expected server.store to be the same as passed store")
	}
}

// Test defaultMetricConfig
func TestDefaultMetricConfig(t *testing.T) {
	config := defaultMetricConfig("test-id")

	if config.ID != "test-id" {
		t.Errorf("Expected ID test-id, got %s", config.ID)
	}

	if config.Window != time.Minute {
		t.Errorf("Expected window 1 minute, got %v", config.Window)
	}

	if config.MaxSamples != 100 {
		t.Errorf("Expected MaxSamples 100, got %d", config.MaxSamples)
	}
}
