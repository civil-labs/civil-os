package main

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is the JSON structure we return
type HealthResponse struct {
	Status       string `json:"status"`
	BackendCount int    `json:"backend_count"`
}

// HealthCheckHandler returns 200 if we have backends, 503 if we don't.
// It takes the BackendManager as a dependency.
func HealthCheckHandler(lb *BackendManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if there is anywhere to send tile server traffic to
		ready := lb.IsReady()

		// Prepare the response
		resp := HealthResponse{
			Status: "OK",
		}

		w.Header().Set("Content-Type", "application/json")

		if ready {
			w.WriteHeader(http.StatusOK) // 200
		} else {
			// Return 503 Service Unavailable if no backends found
			// This tells AWS ALB/ECS to stop routing traffic here until some come up
			w.WriteHeader(http.StatusServiceUnavailable) // 503
			resp.Status = "No tile servers available"
		}

		json.NewEncoder(w).Encode(resp)
	}
}
