# Latency Tracker

A small, focused Go service that tracks **P50 / P95 / P99 latencies** over a **sliding time window**, exposed via simple HTTP APIs.

This project is intentionally scoped as a *weekend backend system* to serve as:

* a crash course in **Go concurrency**
* a reference for **idiomatic net/http servers**
* a concrete example of **systems tradeoffs** (correctness vs simplicity)

---

## What This Service Does

* Accepts latency samples (e.g. request durations)
* Stores them **in memory**, per metric ID
* Computes **P50 / P95 / P99** over a **recent time window**
* Exposes metrics via HTTP in JSON format

Key guarantees:

* Thread-safe under concurrent reads and writes
* Bounded memory usage
* Deterministic percentile calculations

---

## Non-Goals (By Design)

This project does **not**:

* Persist data to disk
* Aggregate across processes
* Use third-party frameworks
* Provide distributed guarantees
* Implement auth or rate limiting

The focus is **clarity and correctness**, not production scale.

---

## High-Level Architecture

```
HTTP Clients
     ↓
net/http Server
     ↓
Server (HTTP layer)
     ↓
MetricStore (map + RWMutex)
     ↓
Metric (samples + Mutex)
```

**Design principle:**

> Writes store facts. Reads apply interpretation.

---

## Core Data Model

### MetricConfig

Static configuration for a metric.

* `ID`: metric identifier
* `Window`: sliding time window
* `MaxSamples`: memory cap

Config is immutable after creation.

---

### LatencySample

Represents one observed latency.

* `LatencyVal`: `time.Duration`
* `RecordedAt`: timestamp

Raw fact data, no aggregation.

---

### Metric

Owns all samples for one metric ID.

* `Samples`: slice of `LatencySample`
* `Mutex`: protects samples

Each metric is independently locked to avoid global contention.

---

### MetricSnapshot

Read-only view returned to clients.

* `Count`: samples in window
* `P50 / P95 / P99`: optional percentile values

Pointers are used to represent **absence of data**.

---

### MetricStore

Registry of all metrics.

* `map[string]*Metric`
* `RWMutex`

Uses `RWMutex` because reads dominate writes.

---

## Core Behavior

### Recording Latency

* Validates input
* Appends `(latency, timestamp)`
* Enforces `MaxSamples`

Write path is **O(1)** and minimally locked.

---

### Calculating Percentiles

Performed **on demand**:

1. Copy samples under lock
2. Filter by time window
3. Sort latencies
4. Compute P50 / P95 / P99

Sliding window logic is applied at **read time**, keeping writes cheap and simple.

---

## HTTP API

### POST /metrics/{id}/latency

Records a latency sample.

Request:

```json
{ "latency_ns": 123 }
// the data here is in nanoseconds
```

Behavior:

* Lazily creates metric if missing
* Records latency
* Returns `202 Accepted`

---

### GET /metrics/{id}

Returns snapshot for a single metric.

---

### GET /metrics

Returns snapshots for **all metrics**.

This is a reporting endpoint and may be computationally heavier.

---

## HTTP Model (Important)

Go HTTP handlers:

```go
func(w http.ResponseWriter, r *http.Request)
```

* Handlers **do not return values**
* Responses are written directly to `ResponseWriter`
* `json.NewEncoder(w).Encode(...)` streams JSON to the client

When the handler returns, the response is finalized.

---

## Concurrency Model

* `MetricStore`: `RWMutex`

  * many readers, rare writers
* `Metric`: `Mutex`

  * protects sample slice

Locks are:

* fine-grained
* short-lived
* never upgraded

---

## Performance Characteristics

* Memory: bounded by `MaxSamples`
* Writes: O(1)
* Reads: O(N log N), where `N ≤ MaxSamples`

Suitable for thousands of samples/sec in a single process.

---

## Server Initialization

1. Create `MetricStore`
2. Create `Server`
3. Register routes via `ServeMux`
4. Start server with `http.ListenAndServe`

The server blocks until terminated.

---

## Why This Project Matters

This codebase demonstrates:

* Correct use of `Mutex` vs `RWMutex`
* Safe map and slice handling
* Realistic HTTP server structure
* Practical tradeoffs in metrics systems

If you can explain this project end-to-end, you understand core Go backend concepts.

---

## Testing

### Test Coverage

The test suite (`server_test.go`) covers:

* **Input validation**: zero/negative latency values, invalid JSON payloads
* **HTTP status codes**: correct responses for success, errors, and edge cases
* **Handler functions**: all endpoints (`/metrics`, `/metrics/{id}`, `/metrics/{id}/latency`)
* **Concurrent access**: multiple goroutines writing to the same metric
* **Edge cases**: empty store, missing metrics, non-existent metric IDs

### Running Tests

From the project root directory:

```bash
# Run all tests with verbose output
go test -v

# Run tests with coverage report
go test -v -cover

# Run tests with detailed coverage per function
go test -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Structure

Tests follow Go conventions:

* Test functions named `Test*` (e.g., `TestHandleRecordLatencyValid`)
* Use `httptest.NewRecorder()` to capture HTTP responses
* Use `httptest.NewRequest()` to create mock requests
* Assertions check status codes, response bodies, and internal state

### Example Test Run

```bash
$ go test -v

=== RUN   TestHandleRecordLatencyValid
--- PASS: TestHandleRecordLatencyValid (0.00s)
=== RUN   TestHandleRecordLatencyZeroValue
--- PASS: TestHandleRecordLatencyZeroValue (0.00s)
=== RUN   TestHandleRecordLatencyConcurrent
--- PASS: TestHandleRecordLatencyConcurrent (0.01s)

ok      latency-tracker 0.042s
```

Tests validate the system's **correctness** and **thread-safety** under realistic conditions.

---

## Possible Extensions (Not Implemented)

* Graceful shutdown
* Configurable windows via API
* Background eviction
* Approximate percentiles
* Persistence

---

## Final Note

This project is intentionally **small, correct, and explainable**.

It is designed to be read, understood, and reasoned about — not just run.
