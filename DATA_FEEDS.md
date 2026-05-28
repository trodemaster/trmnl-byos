# TRMNL Data Feed Requirements

A dedicated JSON feed for the TRMNL display server, served from `wx.jibb.tv`.

## Target endpoint

`http://wx.jibb.tv/trmnl.json`

A new weewx skin template (`trmnl.json.tmpl`) containing only the fields this server needs.

## Required fields

```json
{
    "current": {
        "temperature":     {"value": 71.0, "units": "°F"},
        "humidity":        {"value": 59.0, "units": "%"},
        "barometer":       {"value": 29.98, "units": "inHg"},
        "wind speed":      {"value": 4.0,  "units": "mph"},
        "wind gust":       {"value": 7.0,  "units": "mph"},
        "wind direction":  {"value": 315.0, "units": "°"},
        "rain rate":       {"value": 0.0,  "units": "in/h"}
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

## Weewx template (`trmnl.json.tmpl`)

```
{
    "current":
    {
        #if $current.outTemp.has_data
        "temperature": {"value": $current.outTemp.raw, "units": "$current.outTemp.format(" ").lstrip()"},
        #end if
        #if $current.outHumidity.has_data
        "humidity": {"value": $current.outHumidity.raw, "units": "$current.outHumidity.format(" ").lstrip()"},
        #end if
        #if $current.barometer.has_data
        "barometer": {"value": $current.barometer.raw, "units": "$current.barometer.format(" ").lstrip()"},
        #end if
        #if $current.windSpeed.has_data
        "wind speed": {"value": $current.windSpeed.raw, "units": "$current.windSpeed.format(" ").lstrip()"},
        #end if
        #if $current.windGust.has_data
        "wind gust": {"value": $current.windGust.raw, "units": "$current.windGust.format(" ").lstrip()"},
        #end if
        #if $current.windDir.has_data
        "wind direction": {"value": $current.windDir.raw, "units": "$current.windDir.format(" ").lstrip()"},
        #end if
        #if $current.rainRate.has_data
        "rain rate": {"value": $current.rainRate.raw, "units": "$current.rainRate.format(" ").lstrip()"},
        #end if
        "void_end": null
    },
    "almanac":
    {
        "sunrise": "$almanac.sunrise",
        "sunset":  "$almanac.sunset"
    },
    "generation":
    {
        "time": "$current.dateTime.format("%a, %d %b %Y %H:%M:%S %Z")"
    }
}
```

## Deployment steps on weather station

1. Save the template as `/etc/weewx/skins/JSON/trmnl.json.tmpl` (or wherever the JSON skin lives — check `SKIN_ROOT` in `weewx.conf`).

2. Add an entry to `skin.conf` under `[CheetahGenerator] [[ToDate]]`:
   ```ini
   [[trmnl]]
       template = trmnl.json.tmpl
   ```

3. weewx will generate `trmnl.json` in the skin output directory on the next archive interval (~5 minutes). No restart needed.

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
