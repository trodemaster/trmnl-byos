package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"net/http"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/trodemaster/trmnl-byos/internal/device"
	"github.com/trodemaster/trmnl-byos/internal/plugin"
)

const dataURL = "http://wx.jibb.tv/weewx.json"

type measurement struct {
	Value float64 `json:"value"`
	Units string  `json:"units"`
}

type wxData struct {
	Station struct {
		Location string `json:"location"`
	} `json:"station"`
	Generation struct {
		Time string `json:"time"`
	} `json:"generation"`
	Current struct {
		Temperature measurement `json:"temperature"`
		Humidity    measurement `json:"humidity"`
		Barometer   measurement `json:"barometer"`
		WindSpeed   measurement `json:"wind speed"`
		WindGust    measurement `json:"wind gust"`
		WindDir     measurement `json:"wind direction"`
		RainRate    measurement `json:"rain rate"`
	} `json:"current"`
	Day struct {
		MaxTemp   measurement `json:"max temperature"`
		MinTemp   measurement `json:"min temperature"`
		RainTotal measurement `json:"rain total"`
	} `json:"day"`
}

var (
	hugeFont  font.Face
	medFont   font.Face
	smallFont font.Face
)

func init() {
	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		log.Fatalf("weather: parse font: %v", err)
	}
	const dpi = 96
	hugeFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 253, DPI: dpi})
	medFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 68, DPI: dpi})
	smallFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 36, DPI: dpi})
	plugin.Register(&weatherPlugin{})
}

type weatherPlugin struct{}

func (p *weatherPlugin) Name() string { return "weather" }

func (p *weatherPlugin) Render(_ context.Context, d *device.Device) (*image.Gray, error) {
	w, h := d.Width, d.Height
	if w == 0 {
		w = 1872
	}
	if h == 0 {
		h = 1404
	}

	img := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	margin := w / 24 // ~78px at 1872

	wx, err := fetchWeather()
	if err != nil {
		drawCentered(img, medFont, "Weather unavailable", w/2, h/2-40)
		drawCentered(img, smallFont, err.Error(), w/2, h/2+40)
		return img, nil
	}

	// Header: location left, update time right (12-hour format)
	drawLeft(img, smallFont, wx.Station.Location, margin, h*65/1000)
	drawRight(img, smallFont, formatTime12(wx.Generation.Time), w-margin, h*65/1000)
	drawHLine(img, h*82/1000, margin, w-margin)

	// Temperature — hero element
	drawCentered(img, hugeFont,
		fmt.Sprintf("%.1f%s", wx.Current.Temperature.Value, wx.Current.Temperature.Units),
		w/2, h*37/100)

	// Humidity (left) | Barometer (right)
	drawLeft(img, medFont,
		fmt.Sprintf("Humidity  %.0f%%", wx.Current.Humidity.Value),
		margin, h*55/100)
	drawRight(img, medFont,
		fmt.Sprintf("%.2f inHg", wx.Current.Barometer.Value),
		w-margin, h*55/100)

	// Wind
	drawCentered(img, medFont, windString(wx), w/2, h*65/100)

	// Today's high / low
	drawLeft(img, medFont,
		fmt.Sprintf("High  %.0f%s", wx.Day.MaxTemp.Value, wx.Day.MaxTemp.Units),
		margin, h*75/100)
	drawRight(img, medFont,
		fmt.Sprintf("Low  %.0f%s", wx.Day.MinTemp.Value, wx.Day.MinTemp.Units),
		w-margin, h*75/100)

	// TODO: add Rise/Set row here once almanac data is in the feed (see DATA_FEEDS.md)

	// Rain — only shown when there's something to report
	if wx.Day.RainTotal.Value > 0 {
		rain := fmt.Sprintf("Rain  %.2f in today", wx.Day.RainTotal.Value)
		if wx.Current.RainRate.Value > 0 {
			rain += fmt.Sprintf("  (%.2f in/h)", wx.Current.RainRate.Value)
		}
		drawCentered(img, medFont, rain, w/2, h*85/100)
	}

	return img, nil
}

// formatTime12 parses the weewx generation timestamp and returns it in 12-hour format.
// Input: "Wed, 27 May 2026 18:41:00 PDT" → "6:41 PM PDT"
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

func fetchWeather() (*wxData, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(dataURL)
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

func drawCentered(dst draw.Image, face font.Face, s string, cx, cy int) {
	adv := font.MeasureString(face, s)
	x := cx - adv.Ceil()/2
	metrics := face.Metrics()
	baseline := cy + (metrics.Ascent-metrics.Descent).Ceil()/2
	(&font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}).DrawString(s)
}

func drawLeft(dst draw.Image, face font.Face, s string, x, cy int) {
	metrics := face.Metrics()
	baseline := cy + (metrics.Ascent-metrics.Descent).Ceil()/2
	(&font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}).DrawString(s)
}

func drawRight(dst draw.Image, face font.Face, s string, x, cy int) {
	adv := font.MeasureString(face, s)
	metrics := face.Metrics()
	baseline := cy + (metrics.Ascent-metrics.Descent).Ceil()/2
	(&font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(x-adv.Ceil(), baseline),
	}).DrawString(s)
}

func drawHLine(dst draw.Image, y, x0, x1 int) {
	for x := x0; x <= x1; x++ {
		dst.Set(x, y, color.Black)
	}
}
