# TRMNL BYOS — Claude Instructions

See @DESIGN.md for full protocol, architecture, and plugin documentation.

## Infrastructure

- **Device server (development):** `umac.jibb.tv` — Mac running the Go server during development
- **Weather station:** `wx.jibb.tv` — weewx instance; JSON endpoints:
  - `http://wx.jibb.tv/current_minimal.json` — current conditions (temp, humidity, barometer, wind, rain)
  - `http://wx.jibb.tv/weewx.json` — full dataset
  - `http://wx.jibb.tv/weewx_extended.json` — extended/historical data

## Key conventions

- TRMNL X only — no 1-bit BMP, no model branching, no original TRMNL support
- Plugins return `*image.Gray`; Gray 0 = black ink, 255 = white paper
- `data/devices.json` is runtime state — gitignored, contains API keys and MAC addresses
- `.envrc` is gitignored — set `BASE_URL=http://<host>:8080` for the device to reach the server
- Changing `"plugin"` in `data/devices.json` takes effect on the next device wake — no restart needed (store detects file mtime changes)
