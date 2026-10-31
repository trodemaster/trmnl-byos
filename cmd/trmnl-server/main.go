package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/trodemaster/trmnl-byos/internal/device"
	"github.com/trodemaster/trmnl-byos/server"

	_ "github.com/trodemaster/trmnl-byos/plugins/clock"
	_ "github.com/trodemaster/trmnl-byos/plugins/dashboard"
	_ "github.com/trodemaster/trmnl-byos/plugins/forecast"
	_ "github.com/trodemaster/trmnl-byos/plugins/weather"
)

func main() {
	port := env("PORT", "8080")
	bindAddr := os.Getenv("BIND_ADDR") // empty = all interfaces
	listen := net.JoinHostPort(bindAddr, port)
	baseURL := env("BASE_URL", "http://localhost:"+port)
	dataDir := env("DATA_DIR", "./data")
	debug := os.Getenv("DEBUG") != ""

	store, err := device.NewStore(dataDir)
	if err != nil {
		log.Fatalf("device store: %v", err)
	}

	handler := server.New(store, baseURL, debug)
	if debug {
		log.Printf("listening on %s (base URL: %s) [debug]", listen, baseURL)
	} else {
		log.Printf("listening on %s (base URL: %s)", listen, baseURL)
	}
	log.Fatal(http.ListenAndServe(listen, handler))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
