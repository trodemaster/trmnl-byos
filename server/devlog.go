package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
)

const maxLogBody = 256 << 10

// handleLog receives the device's log uploads (POST /api/log). The firmware
// sends {"logs":[{...},...]} and treats anything but 200/204 as a failure and
// keeps the entries to retry, so a 404 here makes it re-send them forever.
// Each entry is written to the server log so a hang can be diagnosed later.
func (s *Server) handleLog(w http.ResponseWriter, r *http.Request) {
	d, ok := s.store.ByAPIKey(r.Header.Get("Access-Token"))
	if !ok {
		http.Error(w, "unknown device", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxLogBody))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}

	var payload struct {
		Logs []json.RawMessage `json:"logs"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("devlog device=%s unparsed=%q", d.FriendlyID, body)
	}
	for _, entry := range payload.Logs {
		log.Printf("devlog device=%s %s", d.FriendlyID, entry)
	}

	w.WriteHeader(http.StatusNoContent)
}
