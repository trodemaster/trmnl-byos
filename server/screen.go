package server

import (
	"bytes"
	"context"
	"image/png"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/trodemaster/trmnl-byos/internal/device"
	"github.com/trodemaster/trmnl-byos/internal/plugin"
)

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("plugin")

	p, ok := plugin.Get(name)
	if !ok {
		http.Error(w, "unknown plugin: "+name+" (registered: "+strings.Join(plugin.Names(), ", ")+")", http.StatusNotFound)
		return
	}

	d := &device.Device{Width: 1872, Height: 1404}
	img, err := p.Render(context.Background(), d)
	if err != nil {
		log.Printf("preview render error plugin=%s: %v", name, err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, "encode failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Write(buf.Bytes())
}

func (s *Server) handleScreen(w http.ResponseWriter, r *http.Request) {
	screenID := r.PathValue("id")

	d, ok := s.store.ByScreenID(screenID)
	if !ok {
		http.Error(w, "unknown device", http.StatusNotFound)
		return
	}

	p, ok := plugin.Get(d.Plugin)
	if !ok {
		http.Error(w, "plugin not found: "+d.Plugin, http.StatusInternalServerError)
		return
	}

	img, err := p.Render(context.Background(), d)
	if err != nil {
		log.Printf("render error device=%s plugin=%s: %v", screenID, d.Plugin, err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, "encode failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.Write(buf.Bytes())
}
