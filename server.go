// package latencytracker
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	store *MetricStore
}

type recordLatencyRequest struct {
	LatencyNs int64 `json:"latency_ns"`
}

// Constructor for Server struct
func newServer(st *MetricStore) Server {
	return Server{st}
}

func defaultMetricConfig(id string) MetricConfig {
	return MetricConfig{
		ID:         id,
		Window:     time.Minute,
		MaxSamples: 100,
		CreatedAt:  time.Now(),
	}
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// w.WriteHeader(http.StatusOK)
	// w.Write([]byte("The '/metrics' endpoint works!"))

	// below line is not absolutely necessary since in the handlerFunc() for
	// this handler, I've already mentioned it to match with GET type request...
	if r.Method != http.MethodGet {
		http.Error(w, "Only 'GET' requests are processed at this endpoint!", http.StatusMethodNotAllowed)
		// http.StatusMethodNotAllowed is essentially 405
		return
	}

	s.store.mu.RLock()
	metrics := make([]*Metric, 0, len(s.store.Metrics))
	// iterating over the map "Metrics" under MetricStore
	for _, m := range s.store.Metrics {
		metrics = append(metrics, m)
	}
	s.store.mu.RUnlock()

	snapshots := make([]MetricSnapshot, 0, len(metrics))
	for _, m := range metrics {
		snapshots = append(snapshots, m.CalculateLatency())
	}

	w.Header().Set("Content-Type", "application/json")
	// NewEncoder takes a Writer type interface. Since the ResponseWriter also
	// implements Write() function, it can be passed under NewEncoder
	json.NewEncoder(w).Encode(snapshots)
}

// Expected paths:
// "/metrics/{id}"
// "/metrics/{id}/latency"
func (s *Server) handleMetricsById(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	comps := strings.Split(strings.Trim(path, "/"), "/")
	// "comps" is essentially a slcie of components of API, where we get
	// ["metrics", "ID", "latency"], the 3rd one being optional depending
	// upon which endpoint is triggered
	if len(comps) < 2 {
		http.NotFound(w, r)
		return
	}
	id := comps[1]
	if len(comps) == 3 && comps[2] == "latency" && r.Method == http.MethodPost {
		// This is identified as a post request to record latency
		s.handleRecordLatency(w, r, id)
		return
	}
	if len(comps) == 2 && r.Method == http.MethodGet {
		// This is a GET request for querying calculating latncy
		s.handleGetMetric(w, r, id)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleRecordLatency(w http.ResponseWriter, r *http.Request, id string) {
	// fetch latency value from request
	// fetch metric from store, if not present create one
	// call the record latency on the fetched/created metric
	var rq recordLatencyRequest
	if err := json.NewDecoder(r.Body).Decode(&rq); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if rq.LatencyNs <= 0 {
		http.Error(w, "'latency_ns' should not have a negative/zero value", http.StatusBadRequest)
		return
	}

	m := s.store.getOrCreateMetric(id)
	m.RecordLatency(time.Duration(rq.LatencyNs))
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleGetMetric(w http.ResponseWriter, r *http.Request, id string) {
	metric, ok := s.store.getMetric(id)
	if !ok {
		http.Error(w, "Metric not found!", http.StatusNotFound)
		return
	}
	snapshot := metric.CalculateLatency()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}
