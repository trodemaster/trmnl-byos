# TRMNL BYOS — Claude Instructions

See @DESIGN.md for full protocol, architecture, and plugin documentation.

## Infrastructure

- **Device server (development):** `umac.jibb.tv` — Mac running the Go server during development
- **Weather station:** `wx.jibb.tv` — weewx instance; JSON endpoints:
  - `http://wx.jibb.tv/weewx.json` — full dataset (current conditions + day stats); used by `weather` and `dashboard` plugins
  - `http://wx.jibb.tv/trmnl.json` — planned dedicated feed with almanac (sunrise/sunset); see DATA_FEEDS.md
- **Forecast API:** Open-Meteo (`api.open-meteo.com`) — free, no auth, 5-day daily forecast for Seattle (lat 47.6062, lon -122.3321); used by `forecast` and `dashboard` plugins

## Plugins

| Name | Source | Description |
|---|---|---|
| `clock` | local time | Time and date |
| `weather` | wx.jibb.tv | Current conditions: temp, humidity, barometer, wind, high/low, rain |
| `forecast` | Open-Meteo | 5-day forecast: icons, H/L temps, precip probability |
| `dashboard` | both | Current conditions (top 67%) + forecast strip (bottom 33%) |

Active plugin is set in `data/devices.json` → `"plugin": "<name>"`. Currently set to `"dashboard"`.

Preview any plugin without changing device assignment: `GET /preview/<name>`

## Key conventions

- TRMNL X only — no 1-bit BMP, no model branching, no original TRMNL support
- Plugins return `*image.Gray`; Gray 0 = black ink, 255 = white paper
- `data/devices.json` is runtime state — gitignored, contains API keys and MAC addresses
- `.envrc` is gitignored — set `BASE_URL=http://<host>:8080` for the device to reach the server
- Changing `"plugin"` in `data/devices.json` takes effect on the next device wake — no restart needed (store detects file mtime changes)
- `refresh_rate` is 60 seconds; `filename` field rotates each wake to bust the device's SPIFFS cache
