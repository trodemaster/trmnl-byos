package dashboard

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"net/http"
	"os"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/fixed"

	"github.com/trodemaster/trmnl-byos/internal/device"
	"github.com/trodemaster/trmnl-byos/internal/plugin"
)

const (
	lowBatteryThreshold = 3.5 // volts; LiPo ~20% charge

	fcURL = "https://api.open-meteo.com/v1/forecast" +
		"?latitude=47.6062&longitude=-122.3321" +
		"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max" +
		"&temperature_unit=fahrenheit&wind_speed_unit=mph" +
		"&precipitation_unit=inch&timezone=America%2FLos_Angeles" +
		"&forecast_days=5"
)

// wxURL defaults to production; set WX_DATA_URL to point a dev build at a
// test feed (e.g. the weewx-dev Lima VM) without touching the prod binary.
var wxURL = "http://wx.jibb.tv/weewx.json"

// ── Data types ───────────────────────────────────────────────────────────────

type measurement struct {
	Value float64 `json:"value"`
	Units string  `json:"units"`
}

type wxData struct {
	Station    struct{ Location string `json:"location"` } `json:"station"`
	Generation struct{ Time string `json:"time"` }         `json:"generation"`
	Current    struct {
		Temperature measurement `json:"temperature"`
		Humidity    measurement `json:"humidity"`
		Barometer   measurement `json:"barometer"`
		WindSpeed   measurement `json:"wind speed"`
		WindGust    measurement `json:"wind gust"`
		WindDir     measurement `json:"wind direction"`
		RainRate    measurement `json:"rain rate"`
		UV          measurement `json:"uv index"`
		AQI         measurement `json:"pm2_5_nowcast_aqi"`
		IndoorAQI   measurement `json:"pm2_5_in_nowcast_aqi"`
	} `json:"current"`
	Day struct {
		MaxTemp   measurement `json:"max temperature"`
		MinTemp   measurement `json:"min temperature"`
		RainTotal measurement `json:"rain total"`
	} `json:"day"`
}

type forecastResponse struct {
	Daily struct {
		Time                 []string  `json:"time"`
		WeatherCode          []int     `json:"weather_code"`
		TempMax              []float64 `json:"temperature_2m_max"`
		TempMin              []float64 `json:"temperature_2m_min"`
		PrecipProbabilityMax []int     `json:"precipitation_probability_max"`
	} `json:"daily"`
}

// ── Weather icons ────────────────────────────────────────────────────────────
//
// Rasterized (256x256, grayscale+alpha) from Meteocons (basmilius/weather-icons,
// MIT licensed, https://github.com/basmilius/weather-icons), "fill" style —
// gradient-shaded PNGs converted to grayscale, giving real tonal depth instead
// of a flat silhouette. Source SVGs and the conversion command are recorded in
// icons/README.md. wmoCategory's return values are exactly these file names.

//go:embed icons/*.png
var iconFS embed.FS

var iconCache map[string]image.Image

// ── Fonts ────────────────────────────────────────────────────────────────────

var (
	heroFont  font.Face // current temperature
	medFont   font.Face // current conditions data rows + forecast day names
	smallFont font.Face // header text, precip labels
)

func init() {
	if url := os.Getenv("WX_DATA_URL"); url != "" {
		wxURL = url
	}

	iconCache = make(map[string]image.Image, 8)
	for _, name := range []string{"sun", "partly", "cloud", "fog", "drizzle", "rain", "snow", "thunder"} {
		f, err := iconFS.Open("icons/" + name + ".png")
		if err != nil {
			log.Fatalf("dashboard: open icon %s: %v", name, err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			log.Fatalf("dashboard: decode icon %s: %v", name, err)
		}
		iconCache[name] = img
	}

	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		log.Fatalf("dashboard: parse font: %v", err)
	}
	const dpi = 96
	heroFont, _  = opentype.NewFace(tt, &opentype.FaceOptions{Size: 160, DPI: dpi})
	medFont, _   = opentype.NewFace(tt, &opentype.FaceOptions{Size: 54, DPI: dpi})
	smallFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 34, DPI: dpi})
	plugin.Register(&dashPlugin{})
}

type dashPlugin struct{}

func (p *dashPlugin) Name() string { return "dashboard" }

// ── Render ───────────────────────────────────────────────────────────────────

func (p *dashPlugin) Render(_ context.Context, d *device.Device) (*image.Gray, error) {
	w, h := d.Width, d.Height
	if w == 0 {
		w = 1872
	}
	if h == 0 {
		h = 1404
	}

	img := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	margin := w / 24

	// Fetch both sources concurrently.
	type wxResult struct {
		data *wxData
		err  error
	}
	type fcResult struct {
		data *forecastResponse
		err  error
	}
	wxCh := make(chan wxResult, 1)
	fcCh := make(chan fcResult, 1)
	go func() { d, e := fetchWeather(); wxCh <- wxResult{d, e} }()
	go func() { d, e := fetchForecast(); fcCh <- fcResult{d, e} }()
	wxRes := <-wxCh
	fcRes := <-fcCh

	// divY separates current conditions (above) from forecast strip (below).
	// Forecast strip is ~33% of display height.
	divY := h * 670 / 1000 // ≈940px at 1404 height
	fcH := h - divY        // forecast section height ≈464px

	// ── Current conditions ───────────────────────────────────────────────────

	if wxRes.err != nil {
		drawCentered(img, medFont, "Weather unavailable", w/2, h*200/1000)
		drawCentered(img, smallFont, wxRes.err.Error(), w/2, h*270/1000)
	} else {
		wx := wxRes.data
		drawLeft(img, smallFont, wx.Station.Location, margin, h*50/1000)
		drawRight(img, smallFont, formatTime12(wx.Generation.Time), w-margin, h*50/1000)
		drawHLine(img, h*71/1000, margin, w-margin)

		drawCentered(img, heroFont,
			fmt.Sprintf("%.1f%s", wx.Current.Temperature.Value, wx.Current.Temperature.Units),
			w/2, h*255/1000)

		drawLeft(img, medFont,
			fmt.Sprintf("Humidity  %.0f%%", wx.Current.Humidity.Value),
			margin, h*380/1000)
		drawRight(img, medFont,
			fmt.Sprintf("%.2f inHg", wx.Current.Barometer.Value),
			w-margin, h*380/1000)

		drawLeft(img, medFont, windString(wx), margin, h*455/1000)
		drawRight(img, medFont,
			fmt.Sprintf("UV %.0f", wx.Current.UV.Value),
			w-margin, h*455/1000)

		drawLeft(img, medFont,
			fmt.Sprintf("High  %.0f%s", wx.Day.MaxTemp.Value, wx.Day.MaxTemp.Units),
			margin, h*525/1000)
		drawRight(img, medFont,
			fmt.Sprintf("Low  %.0f%s", wx.Day.MinTemp.Value, wx.Day.MinTemp.Units),
			w-margin, h*525/1000)

		// AQI has_data is only omitted from the feed if the AirLink sensor is
		// unreachable; Units is empty in that case since 0 is a valid AQI value.
		if wx.Current.AQI.Units != "" {
			drawLeft(img, medFont,
				fmt.Sprintf("AQI  %.0f", wx.Current.AQI.Value),
				margin, h*585/1000)
		}
		if wx.Current.IndoorAQI.Units != "" {
			drawRight(img, medFont,
				fmt.Sprintf("Indoor AQI  %.0f", wx.Current.IndoorAQI.Value),
				w-margin, h*585/1000)
		}

		if wx.Day.RainTotal.Value > 0 {
			rain := fmt.Sprintf("Rain  %.2f in today", wx.Day.RainTotal.Value)
			if wx.Current.RainRate.Value > 0 {
				rain += fmt.Sprintf("  (%.2f in/h)", wx.Current.RainRate.Value)
			}
			drawCentered(img, smallFont, rain, w/2, h*628/1000)
		}

		if d.BatteryVoltage > 0 && d.BatteryVoltage < lowBatteryThreshold {
			drawRight(img, smallFont,
				fmt.Sprintf("LOW BATTERY  %.2fV", d.BatteryVoltage),
				w-margin, h*654/1000)
		}
	}

	// Section divider (3px thick, full width)
	for dy := 0; dy < 3; dy++ {
		drawHLine(img, divY+dy, 0, w)
	}

	// ── Forecast strip ───────────────────────────────────────────────────────

	if fcRes.err != nil {
		drawCentered(img, medFont, "Forecast unavailable", w/2, divY+fcH/2)
	} else {
		fc := fcRes.data
		n := len(fc.Daily.Time)
		if n > 5 {
			n = 5
		}

		colW := (w - 2*margin) / n
		iconR := colW * 19 / 100 // smaller icon for the compact strip

		for i := 0; i < n; i++ {
			cx := margin + colW*i + colW/2

			t, _ := time.Parse("2006-01-02", fc.Daily.Time[i])
			dayName := t.Format("Mon")
			if i == 0 {
				dayName = "Today"
			}
			drawCentered(img, medFont, dayName, cx, divY+fcH*145/1000)

			if i < len(fc.Daily.WeatherCode) {
				drawWeatherIcon(img, cx, divY+fcH*327/1000, iconR, fc.Daily.WeatherCode[i])
			}

			if i < len(fc.Daily.TempMax) {
				drawCentered(img, medFont,
					fmt.Sprintf("H %.0f°", fc.Daily.TempMax[i]),
					cx, divY+fcH*618/1000)
			}
			if i < len(fc.Daily.TempMin) {
				drawCentered(img, medFont,
					fmt.Sprintf("L %.0f°", fc.Daily.TempMin[i]),
					cx, divY+fcH*775/1000)
			}
			if i < len(fc.Daily.PrecipProbabilityMax) {
				drawCentered(img, smallFont,
					fmt.Sprintf("%d%% rain", fc.Daily.PrecipProbabilityMax[i]),
					cx, divY+fcH*900/1000)
			}

			if i < n-1 {
				x := margin + colW*(i+1)
				for y := divY + 3; y < h; y++ {
					img.SetGray(x, y, color.Gray{0})
				}
			}
		}
	}

	return img, nil
}

// ── Data fetchers ────────────────────────────────────────────────────────────

func fetchWeather() (*wxData, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(wxURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var wx wxData
	if err := json.NewDecoder(resp.Body).Decode(&wx); err != nil {
		return nil, err
	}
	return &wx, nil
}

func fetchForecast() (*forecastResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fcURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var fc forecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return nil, err
	}
	return &fc, nil
}

// ── Current conditions helpers ───────────────────────────────────────────────

func formatTime12(s string) string {
	t, err := time.Parse("Mon, 02 Jan 2006 15:04:05 MST", s)
	if err != nil {
		return s
	}
	return t.Format("Mon Jan 2  3:04 PM")
}

func windString(wx *wxData) string {
	if wx.Current.WindSpeed.Value < 1 {
		return "Calm"
	}
	s := fmt.Sprintf("%s  %.0f mph", compassDir(wx.Current.WindDir.Value), wx.Current.WindSpeed.Value)
	if wx.Current.WindGust.Value > wx.Current.WindSpeed.Value {
		s += fmt.Sprintf("  (gusts %.0f)", wx.Current.WindGust.Value)
	}
	return s
}

func compassDir(deg float64) string {
	dirs := [16]string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	return dirs[int(math.Round(deg/22.5))%16]
}

// ── WMO / icon helpers ───────────────────────────────────────────────────────

func wmoCategory(code int) string {
	switch {
	case code == 0 || code == 1:
		return "sun"
	case code == 2:
		return "partly"
	case code == 3:
		return "cloud"
	case code == 45 || code == 48:
		return "fog"
	case code >= 51 && code <= 57:
		return "drizzle"
	case (code >= 61 && code <= 67) || (code >= 80 && code <= 82):
		return "rain"
	case (code >= 71 && code <= 77) || (code >= 85 && code <= 86):
		return "snow"
	default:
		return "thunder"
	}
}

// drawWeatherIcon composites the Meteocons PNG for the WMO code's category
// into a (2r x 2r) square centered at (cx, cy), scaling with CatmullRom
// (quality bicubic — these are small raster images, not vector, so scaling
// quality matters) and alpha-compositing over whatever is already drawn.
func drawWeatherIcon(img *image.Gray, cx, cy, r, code int) {
	src, ok := iconCache[wmoCategory(code)]
	if !ok {
		return
	}
	dr := image.Rect(cx-r, cy-r, cx+r, cy+r)
	xdraw.CatmullRom.Scale(img, dr, src, src.Bounds(), xdraw.Over, nil)
}

// ── Text helpers ──────────────────────────────────────────────────────────────

func drawCentered(dst draw.Image, face font.Face, s string, cx, cy int) {
	adv := font.MeasureString(face, s)
	x := cx - adv.Ceil()/2
	metrics := face.Metrics()
	baseline := cy + (metrics.Ascent-metrics.Descent).Ceil()/2
	(&font.Drawer{Dst: dst, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P(x, baseline)}).DrawString(s)
}

func drawLeft(dst draw.Image, face font.Face, s string, x, cy int) {
	metrics := face.Metrics()
	baseline := cy + (metrics.Ascent-metrics.Descent).Ceil()/2
	(&font.Drawer{Dst: dst, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P(x, baseline)}).DrawString(s)
}

func drawRight(dst draw.Image, face font.Face, s string, x, cy int) {
	adv := font.MeasureString(face, s)
	metrics := face.Metrics()
	baseline := cy + (metrics.Ascent-metrics.Descent).Ceil()/2
	(&font.Drawer{Dst: dst, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P(x-adv.Ceil(), baseline)}).DrawString(s)
}

func drawHLine(img *image.Gray, y, x0, x1 int) {
	for x := x0; x <= x1; x++ {
		img.SetGray(x, y, color.Gray{Y: 0})
	}
}
