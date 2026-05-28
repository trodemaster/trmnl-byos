package server

import (
	"net/http"

	"github.com/trodemaster/trmnl-byos/internal/device"
)

type Server struct {
	store   *device.Store
	baseURL string
}

func New(store *device.Store, baseURL string) http.Handler {
	s := &Server{store: store, baseURL: baseURL}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/setup", s.handleSetup)
	mux.HandleFunc("GET /api/display", s.handleDisplay)
	mux.HandleFunc("GET /screen/{id}", s.handleScreen)

	return mux
}
