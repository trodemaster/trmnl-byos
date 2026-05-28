# Weather Data Feed Requirements

## Current Feeds

**Base URL:** `http://wx.jibb.tv`

| Endpoint | Description | Status |
|---|---|---|
| `/current_minimal.json` | Current conditions (temp, humidity, barometer, wind, rain rate) | Live |
| `/weewx.json` | Full dataset including day/week/month/year stats | Live |
| `/weewx_extended.json` | Extended data | 404 — not deployed |

## Required Additions

### Almanac data in `current_minimal.json`

The weather plugin at `plugins/weather/weather.go` fetches `current_minimal.json`. The following fields need to be added to the `current_minimal.json.tmpl` skin template on the weather station:

```json
"almanac": {
    "sunrise": "5:18 AM",
    "sunset": "8:55 PM"
}
```

**Weewx template variables** (Cheetah syntax, to be added inside `current_minimal.json.tmpl`):

```
"almanac":
{
    "sunrise": "$almanac.sunrise",
    "sunset":  "$almanac.sunset"
}
```

These values are already computed by weewx and displayed on the HTML pages at `http://wx.jibb.tv/` and `http://wx.jibb.tv/almanac.html`. They just need to be surfaced in the JSON feed.

### Template file location on weather station

`/etc/weewx/skins/JSON/current_minimal.json.tmpl`

(Or wherever the weewx skin is installed — confirm with `weewx config` or check the weewx config file for `SKIN_ROOT`.)

After editing the template, weewx will pick up the change on the next archive interval (typically 5 minutes) without requiring a restart.

## Go plugin consuming this data

`plugins/weather/weather.go` — the `wxData` struct will need a new `Almanac` field once the feed is updated:

```go
type wxData struct {
    // ... existing fields ...
    Almanac struct {
        Sunrise string `json:"sunrise"`
        Sunset  string `json:"sunset"`
    } `json:"almanac"`
}
```

The time strings (`"5:18 AM"`) can be parsed with `time.Parse("3:04 PM", wx.Almanac.Sunrise)`.
