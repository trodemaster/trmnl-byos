package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	mac := r.Header.Get("ID")
	if mac == "" {
		http.Error(w, "missing ID header", http.StatusBadRequest)
		return
	}

	d := s.store.GetOrCreate(mac, r.Header.Get("Model"), r.Header.Get("FW-Version"))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":      200,
		"api_key":     d.APIKey,
		"friendly_id": d.FriendlyID,
		"image_url":   fmt.Sprintf("%s/screen/%s", s.baseURL, d.ScreenID),
		"message":     "",
	})
}
