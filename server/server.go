package server

import (
	"net/http"

	"github.com/trodemaster/trmnl-byos/internal/device"
)

type Server struct {
	store   *device.Store
	baseURL string
	debug   bool
}

func New(store *device.Store, baseURL string, debug bool) http.Handler {
	s := &Server{store: store, baseURL: baseURL, debug: debug}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/setup", s.handleSetup)
	mux.HandleFunc("GET /api/display", s.handleDisplay)
	mux.HandleFunc("GET /screen/{id}", s.handleScreen)
	mux.HandleFunc("GET /preview/{plugin}", s.handlePreview)

	return logMiddleware(mux)
}
