package main

import (
	"encoding/json"
	"net/http"
)

// HealthCheckHandler returns 200 for now. In the future may
// do further health introspection to downstream services
func HealthCheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK) // 200

		json.NewEncoder(w).Encode(resp)
	}
}
