GOPATH        := $(shell go env GOPATH)
BINARY_PROD   := $(GOPATH)/bin/trmnl-server
BINARY_DEV    := $(GOPATH)/bin/trmnl-server-dev
LAUNCH_AGENTS := $(HOME)/Library/LaunchAgents
DOMAIN        := gui/$(shell id -u)
PLIST_PROD    := tv.jibb.trmnl-byos
PLIST_DEV     := tv.jibb.trmnl-byos-dev

.PHONY: build build-clean install reinstall uninstall \
        dev dev-install dev-reinstall dev-start dev-stop dev-uninstall \
        status

# ── Production ────────────────────────────────────────────────────────────────

# Fast build (no cache clean) — use for quick iteration
build:
	go install ./cmd/trmnl-server

# Reliable build — cleans cache first to work around MacPorts Go cache corruption
build-clean:
	go clean -cache
	go install ./cmd/trmnl-server

# First-time install or after Go/plist changes: clean build + load launchd service
install: build-clean
	cp LaunchAgents/$(PLIST_PROD).plist $(LAUNCH_AGENTS)/
	launchctl bootout $(DOMAIN) $(LAUNCH_AGENTS)/$(PLIST_PROD).plist 2>/dev/null || true
	launchctl bootstrap $(DOMAIN) $(LAUNCH_AGENTS)/$(PLIST_PROD).plist
	@echo "prod service started on :8080"

# Fast update of running prod service (skips cache clean)
reinstall: build
	launchctl kickstart -k $(DOMAIN)/$(PLIST_PROD)
	@echo "prod service restarted on :8080"

uninstall:
	launchctl bootout $(DOMAIN) $(LAUNCH_AGENTS)/$(PLIST_PROD).plist 2>/dev/null || true
	rm -f $(LAUNCH_AGENTS)/$(PLIST_PROD).plist
	@echo "prod service removed"

# ── Dev ───────────────────────────────────────────────────────────────────────

# Build and run dev server in the foreground on :8081 (Ctrl-C to stop)
dev:
	go build -o $(BINARY_DEV) ./cmd/trmnl-server
	BASE_URL=http://192.168.234.144:8081 PORT=8081 $(BINARY_DEV)

# Register dev launchd service (does not start it — use dev-start)
dev-install:
	go build -o $(BINARY_DEV) ./cmd/trmnl-server
	cp LaunchAgents/$(PLIST_DEV).plist $(LAUNCH_AGENTS)/
	launchctl bootout $(DOMAIN) $(LAUNCH_AGENTS)/$(PLIST_DEV).plist 2>/dev/null || true
	launchctl bootstrap $(DOMAIN) $(LAUNCH_AGENTS)/$(PLIST_DEV).plist
	@echo "dev service registered — run 'make dev-start' to launch"

# Rebuild dev binary and (re)start background service
dev-reinstall:
	go build -o $(BINARY_DEV) ./cmd/trmnl-server
	launchctl kickstart -k $(DOMAIN)/$(PLIST_DEV)
	@echo "dev service restarted on :8081"

dev-start:
	launchctl kickstart $(DOMAIN)/$(PLIST_DEV)
	@echo "dev service started on :8081"

dev-stop:
	launchctl kill SIGTERM $(DOMAIN)/$(PLIST_DEV)
	@echo "dev service stopped"

dev-uninstall:
	launchctl bootout $(DOMAIN) $(LAUNCH_AGENTS)/$(PLIST_DEV).plist 2>/dev/null || true
	rm -f $(LAUNCH_AGENTS)/$(PLIST_DEV).plist
	@echo "dev service removed"

# ── Status ────────────────────────────────────────────────────────────────────

status:
	@echo "=== launchd services ==="
	@launchctl list | grep trmnl || echo "(none)"
	@echo ""
	@echo "=== HTTP smoke tests ==="
	@curl -sf --max-time 2 http://localhost:8080/preview/clock > /dev/null \
		&& echo "prod :8080  OK" || echo "prod :8080  not responding"
	@curl -sf --max-time 2 http://localhost:8081/preview/clock > /dev/null \
		&& echo "dev  :8081  OK" || echo "dev  :8081  not responding"
