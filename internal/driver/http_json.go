package driver

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func writeDriverJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("driver write json failed", "status", status, "error", err)
	}
}
