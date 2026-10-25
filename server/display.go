package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/trodemaster/trmnl-byos/internal/device"
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
	batt, _ := strconv.ParseFloat(r.Header.Get("Battery-Voltage"), 64)
	pct, _ := strconv.ParseFloat(r.Header.Get("Percent-Charged"), 64)
	// Battery-Charging is "1"/"0" in current firmware; older docs say "true".
	charging := r.Header.Get("Battery-Charging") == "1" || r.Header.Get("Battery-Charging") == "true"
	usb := r.Header.Get("USB-Connected") == "true"
	s.store.UpdateDeviceInfo(apiKey, r.Header.Get("Model"), r.Header.Get("FW-Version"), width, height,
		device.Battery{Voltage: batt, Percent: pct, Charging: charging, USB: usb})

	if batt > 0 {
		log.Printf("wake device=%s plugin=%s fw=%s batt=%.2fV pct=%.0f%% charging=%t usb=%t current=%smA",
			d.FriendlyID, d.Plugin, d.FWVersion, batt, pct, charging, usb, r.Header.Get("Battery-Current"))
	} else {
		log.Printf("wake device=%s plugin=%s fw=%s", d.FriendlyID, d.Plugin, d.FWVersion)
	}
	if s.debug {
		log.Printf("wake detail device=%s rssi=%s wake-time=%sms cached=%s",
			d.FriendlyID,
			r.Header.Get("RSSI"),
			r.Header.Get("wake-time"),
			r.Header.Get("image-cached"),
		)
	}

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
