// package latencyTracker
package main

import (
	"log"
	"net/http"
)

func main() {
	store := &MetricStore{}
	server := Server{store}

	router := http.NewServeMux()
	router.HandleFunc("GET /metrics", server.handleMetrics)
	// Golang, more specifically ServerMux inherently cannot handle path
	// parameters, so we pass either prefix/suffix of an endpoint and later
	// handle it inside the handler which is passed in "HandleFunc()"
	router.HandleFunc("/metrics/", server.handleMetricsById)

	log.Println("Listening on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", router))
}
