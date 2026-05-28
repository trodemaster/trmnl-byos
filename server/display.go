package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleDisplay(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("Access-Token")
	d, ok := s.store.ByAPIKey(apiKey)
	if !ok {
		http.Error(w, "unknown device", http.StatusUnauthorized)
		return
	}

	width, _ := strconv.Atoi(r.Header.Get("Width"))
	height, _ := strconv.Atoi(r.Header.Get("Height"))
	s.store.UpdateDeviceInfo(apiKey, r.Header.Get("Model"), r.Header.Get("FW-Version"), width, height)

	// Cache-bust with a timestamp so the device always fetches a fresh image.
	imageURL := fmt.Sprintf("%s/screen/%s?t=%d", s.baseURL, d.ScreenID, time.Now().Unix())

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":          0,
		"image_url":       imageURL,
		"filename":        fmt.Sprintf("%s-%d.png", d.ScreenID, time.Now().Unix()),
		"refresh_rate":    60,
		"update_firmware": false,
		"reset_firmware":  false,
	})
}
