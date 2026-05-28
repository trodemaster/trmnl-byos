package main

import (
	"log"
	"net/http"
	"os"

	"github.com/trodemaster/trmnl-byos/internal/device"
	"github.com/trodemaster/trmnl-byos/server"

	_ "github.com/trodemaster/trmnl-byos/plugins/clock"
	_ "github.com/trodemaster/trmnl-byos/plugins/forecast"
	_ "github.com/trodemaster/trmnl-byos/plugins/weather"
)

func main() {
	port := env("PORT", "8080")
	baseURL := env("BASE_URL", "http://localhost:"+port)
	dataDir := env("DATA_DIR", "./data")

	store, err := device.NewStore(dataDir)
	if err != nil {
		log.Fatalf("device store: %v", err)
	}

	handler := server.New(store, baseURL)
	log.Printf("listening on :%s (base URL: %s)", port, baseURL)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
