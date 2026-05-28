package device

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TRMNL X default display dimensions. The device overwrites these on first /api/display call.
const (
	DefaultWidth  = 1872
	DefaultHeight = 1404
)

type Device struct {
	ID         string    `json:"id"`          // original MAC address from firmware
	ScreenID   string    `json:"screen_id"`   // URL-safe hex ID (MAC without colons)
	APIKey     string    `json:"api_key"`
	FriendlyID string    `json:"friendly_id"`
	Model      string    `json:"model"`
	FWVersion  string    `json:"fw_version"`
	Plugin     string    `json:"plugin"` // active plugin name
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	LastSeen   time.Time `json:"last_seen"`
}

type Store struct {
	mu      sync.RWMutex
	devices map[string]*Device // keyed by ScreenID
	byKey   map[string]*Device // keyed by APIKey
	path    string
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		devices: make(map[string]*Device),
		byKey:   make(map[string]*Device),
		path:    filepath.Join(dataDir, "devices.json"),
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	_ = s.load()
	return s, nil
}

func (s *Store) GetOrCreate(mac, model, fwVersion string) *Device {
	s.mu.Lock()
	defer s.mu.Unlock()

	screenID := normalizeMAC(mac)
	if d, ok := s.devices[screenID]; ok {
		d.Model = model
		d.FWVersion = fwVersion
		d.LastSeen = time.Now()
		s.save()
		return d
	}

	key := randomKey()
	d := &Device{
		ID:         mac,
		ScreenID:   screenID,
		APIKey:     key,
		FriendlyID: "device-" + screenID[len(screenID)-4:],
		Model:      model,
		FWVersion:  fwVersion,
		Plugin:     "clock",
		Width:      DefaultWidth,
		Height:     DefaultHeight,
		LastSeen:   time.Now(),
	}
	s.devices[screenID] = d
	s.byKey[key] = d
	s.save()
	return d
}

func (s *Store) ByAPIKey(key string) (*Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.byKey[key]
	return d, ok
}

func (s *Store) ByScreenID(screenID string) (*Device, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.devices[screenID]
	return d, ok
}

func (s *Store) UpdateDeviceInfo(apiKey, model, fwVersion string, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.byKey[apiKey]
	if !ok {
		return
	}
	if model != "" {
		d.Model = model
	}
	if fwVersion != "" {
		d.FWVersion = fwVersion
	}
	if width > 0 {
		d.Width = width
	}
	if height > 0 {
		d.Height = height
	}
	d.LastSeen = time.Now()
	s.save()
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	var devices []*Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return err
	}
	for _, d := range devices {
		s.devices[d.ScreenID] = d
		s.byKey[d.APIKey] = d
	}
	return nil
}

func (s *Store) save() {
	devices := make([]*Device, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, d)
	}
	data, _ := json.MarshalIndent(devices, "", "  ")
	_ = os.WriteFile(s.path, data, 0o644)
}

// normalizeMAC strips colons/hyphens and lowercases: "AA:BB:CC:DD:EE:FF" → "aabbccddeeff"
func normalizeMAC(mac string) string {
	return strings.ToLower(strings.NewReplacer(":", "", "-", "").Replace(mac))
}

func randomKey() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
