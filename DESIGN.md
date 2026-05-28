# TRMNL BYOS — Design & Implementation

A Go server implementing the TRMNL firmware's BYOS (Bring Your Own Server) protocol, targeting the TRMNL X device exclusively.

## Device

**TRMNL X** — 10.3" e-ink display, 16-level grayscale, ESP32-S3 microcontroller. The firmware sends model string `"x"` in HTTP headers. Default resolution 1872×1404, but the device reports its actual dimensions in the `Width`/`Height` headers on every `/api/display` call.

The device wakes on a timer, makes two HTTP requests (display metadata then the image), renders the image, and goes back to sleep. The server has no persistent connection to the device — it is purely pull-based.

## Protocol

Derived directly from reading the firmware source (`src/api-client/display.cpp`, `src/api-client/setup.cpp`, `lib/trmnl/src/parse_response_api_display.cpp`).

### First boot — `GET /api/setup`

Device sends:

| Header | Value |
|---|---|
| `ID` | MAC address (e.g. `AA:BB:CC:DD:EE:FF`) |
| `Model` | `x` for TRMNL X |
| `FW-Version` | firmware version string |

Server responds with JSON:

```json
{
  "status": 200,
  "api_key": "<random 32-hex token>",
  "friendly_id": "device-eeff",
  "image_url": "http://<server>/screen/<screenID>",
  "message": ""
}
```

The device stores `api_key` and `friendly_id` in flash and does not call `/api/setup` again unless factory reset.

### Every wake — `GET /api/display`

Device sends:

| Header | Value |
|---|---|
| `Access-Token` | the stored `api_key` |
| `Width` | display width in pixels |
| `Height` | display height in pixels |
| `Model` | `x` |
| `FW-Version` | firmware version string |
| `Battery-Voltage` | float |
| `Refresh-Rate` | current sleep interval (ms) |
| `RSSI` | WiFi signal strength |
| `wake-time` | duration of previous wake cycle (ms) |
| `image-cached` | `true` if last image was served from device cache |

Server responds with JSON:

```json
{
  "status": 0,
  "image_url": "http://<server>/screen/<screenID>?t=<unix-timestamp>",
  "refresh_rate": 900,
  "update_firmware": false,
  "reset_firmware": false
}
```

`refresh_rate` is in seconds. The timestamp query param is a cache-buster — without it the device may skip downloading if it thinks it already has the image.

### Image fetch — `GET /screen/<screenID>`

Device downloads whatever URL was in `image_url`. The server renders the active plugin for that device and responds with a PNG image (grayscale, matching the device's reported dimensions).

The TRMNL X firmware detects PNG by checking the `0x89504e47` magic bytes and routes to its PNG decoder. It supports grayscale PNG natively, which gives the full 16-level range of the e-ink display.

## Image format

**PNG, grayscale (`image.Gray`).** The plugin `Render` method returns `*image.Gray` (stdlib), which Go's `image/png` encodes directly. Gray value 0 = black ink, 255 = white paper.

The original TRMNL (model `"og"`) uses 1-bit BMP, but this server targets TRMNL X only so that complexity is excluded.

## Project layout

```
trmnl-byos/
├── cmd/server/main.go          entry point; wires store → server; reads env vars
├── internal/
│   ├── device/store.go         Device struct + JSON-backed registry
│   └── plugin/plugin.go        Plugin interface + global registry
├── server/
│   ├── server.go               http.ServeMux wiring
│   ├── setup.go                GET /api/setup
│   ├── display.go              GET /api/display
│   └── screen.go               GET /screen/{id} — renders plugin → PNG
└── plugins/
    └── clock/clock.go          built-in example plugin
```

## Plugin system

Plugins register themselves via `init()` using a package-level registry in `internal/plugin`. The server never imports plugins directly — `main.go` does, with blank imports to trigger `init()`.

```go
// internal/plugin/plugin.go
type Plugin interface {
    Name() string
    Render(ctx context.Context, d *device.Device) (*image.Gray, error)
}
```

`d.Width` and `d.Height` are populated from device-reported headers before `Render` is called, so plugins should always use them rather than hardcoding dimensions.

### Adding a plugin

1. Create `plugins/<name>/<name>.go` in package `<name>`.
2. Implement `Plugin`, call `plugin.Register` in `init()`.
3. Add a blank import to `cmd/server/main.go`: `_ "github.com/trodemaster/trmnl-byos/plugins/<name>"`.
4. Edit `data/devices.json` and set `"plugin": "<name>"` for the target device.

### Changing a device's plugin at runtime

Edit `data/devices.json` directly. Changes take effect on the next wake cycle — no server restart needed because the store reads from the JSON file on every request.

## Device store

`internal/device/Store` is an in-memory map backed by `data/devices.json`. It holds two indexes:

- `devices` — keyed by `ScreenID` (MAC with colons stripped, lowercased: `aabbccddeeff`)
- `byKey` — keyed by `APIKey`

MAC addresses are normalized on ingestion so routing is stable regardless of how the firmware formats the `ID` header. The file is written synchronously on every mutation (create, update); suitable for a single device.

`data/devices.json` example:

```json
[
  {
    "id": "AA:BB:CC:DD:EE:FF",
    "screen_id": "aabbccddeeff",
    "api_key": "b0df9626a85ceac023408b2011b15cfd",
    "friendly_id": "device-eeff",
    "model": "x",
    "fw_version": "2.0",
    "plugin": "clock",
    "width": 1872,
    "height": 1404,
    "last_seen": "2026-05-27T17:41:59Z"
  }
]
```

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `BASE_URL` | `http://localhost:<PORT>` | Externally reachable URL of this server — embedded in `image_url` responses, so the device must be able to reach it |
| `DATA_DIR` | `./data` | Directory for `devices.json` |

`BASE_URL` must be set to your server's LAN IP (e.g. `http://192.168.1.100:8080`) when running on a local network, otherwise the device cannot download the image.

## Dependencies

- `golang.org/x/image` — OpenType font parsing (`font/opentype`), Go Regular font (`font/gofont/goregular`), font drawing primitives (`font`, `math/fixed`)
- Everything else is Go stdlib (`image`, `image/png`, `image/draw`, `net/http`, `encoding/json`)

## Running

```bash
BASE_URL=http://192.168.1.100:8080 go run ./cmd/server
```

Build a binary:

```bash
go build -o trmnl-server ./cmd/server
BASE_URL=http://192.168.1.100:8080 ./trmnl-server
```

Point the TRMNL X at your server URL in the device's network/server settings. On first boot it will call `/api/setup`; subsequent wakes call `/api/display` → download image → render → sleep.
