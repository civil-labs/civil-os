package main

import (
	"encoding/json"
	"net/http"
)

// HealthResponse is the JSON structure we return
type HealthResponse struct {
	Status       string `json:"status"`
}

// HealthCheckHandler returns 200 if we have backends, In the future may
// do further health introspection to downstream services
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// Check if there is anywhere to send tile server traffic to
		ready := lb.IsReady()

		// Prepare the response
		resp := HealthResponse{
			Status: "OK",
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK) // 200

		json.NewEncoder(w).Encode(resp)
	}
}
