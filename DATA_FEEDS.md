# TRMNL Data Feed Requirements

A dedicated JSON feed for the TRMNL display server, served from `wx.jibb.tv`.

## Target endpoint

`http://wx.jibb.tv/trmnl.json`

A new weewx skin template (`trmnl.json.tmpl`) containing only the fields this server needs.

## Available fields

The live feed (`trmnl.json`) exposes the fields below. The Go plugin only decodes the
subset it renders; extra fields are ignored, so the feed can carry more than the display uses.

```json
{
    "station": {
        "location": "West Woodland"
    },
    "current": {
        "temperature":         {"value": 71.0, "units": "°F"},
        "humidity":            {"value": 59.0, "units": "%"},
        "barometer":           {"value": 29.98, "units": "inHg"},
        "wind speed":          {"value": 4.0,  "units": "mph"},
        "wind gust":           {"value": 7.0,  "units": "mph"},
        "wind direction":      {"value": 315.0, "units": "°"},
        "rain rate":           {"value": 0.0,  "units": "in/h"},
        "dew point":           {"value": 52.0, "units": "°F"},
        "wind chill":          {"value": 70.0, "units": "°F"},
        "heat index":          {"value": 71.0, "units": "°F"},
        "inside temperature":  {"value": 68.0, "units": "°F"},
        "inside humidity":     {"value": 45.0, "units": "%"},
        "uv index":            {"value": 5.0,  "units": ""},
        "radiation":           {"value": 480.0, "units": "W/m²"},
        "cloud base":          {"value": 4200.0, "units": "feet"},
        "aqi nowcast":         {"value": 28.0, "units": "AQI"}
    },
    "day": {
        "max temperature": {"value": 72.0, "units": "°F"},
        "min temperature": {"value": 48.5, "units": "°F"},
        "rain total":      {"value": 0.0,  "units": "in"}
    },
    "almanac": {
        "sunrise": "5:18 AM",
        "sunset":  "8:55 PM"
    },
    "generation": {
        "time": "Wed, 27 May 2026 18:53:00 PDT"
    }
}
```

Each `current`/`day` field is conditional on `has_data` in weewx, so a field is absent
(not null) when the station has no value for it. `aqi nowcast` comes from the AirLink
sensor's NowCast PM2.5, computed with the 2026 EPA breakpoints.

## Weewx template

The canonical template lives in the `weewx-json` repo at
`skins/JSON/trmnl.json.tmpl` and is registered as `[[trmnl]]` in that skin's `skin.conf`.
It is installed as part of the weewx-json extension. Do not maintain a copy here.

## Deployment steps on weather station

The template ships with the weewx-json extension, so deploying is just installing/upgrading it:

1. Install or upgrade the weewx-json extension on wx (`weectl extension install`), which
   places `trmnl.json.tmpl` under the JSON skin and registers the `[[trmnl]]` report.
2. weewx generates `trmnl.json` in the skin output directory on the next archive interval.
   Confirm `http://wx.jibb.tv/trmnl.json` is live before switching the Go plugin (below).

## Go plugin update (after feed is live)

Update `dataURL` in `plugins/weather/weather.go`:
```go
const dataURL = "http://wx.jibb.tv/trmnl.json"
```

Add `Almanac` to the `wxData` struct:
```go
Almanac struct {
    Sunrise string `json:"sunrise"`
    Sunset  string `json:"sunset"`
} `json:"almanac"`
```

Then add the Rise/Set row to the render function (the `TODO` comment is already in place).
