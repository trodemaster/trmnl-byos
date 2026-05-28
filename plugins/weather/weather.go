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

const dataURL = "http://wx.jibb.tv/current_minimal.json"

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
	hugeFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 180, DPI: dpi})
	medFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 52, DPI: dpi})
	smallFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 32, DPI: dpi})
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

	// Header: location left, update time right
	drawLeft(img, smallFont, wx.Station.Location, margin, h*65/1000)
	drawRight(img, smallFont, wx.Generation.Time, w-margin, h*65/1000)
	drawHLine(img, h*82/1000, margin, w-margin)

	// Temperature — hero element
	drawCentered(img, hugeFont,
		fmt.Sprintf("%.1f%s", wx.Current.Temperature.Value, wx.Current.Temperature.Units),
		w/2, h*42/100)

	// Humidity (left) | Barometer (right)
	drawLeft(img, medFont,
		fmt.Sprintf("Humidity  %.0f%%", wx.Current.Humidity.Value),
		w/4, h*60/100)
	drawRight(img, medFont,
		fmt.Sprintf("%.2f inHg", wx.Current.Barometer.Value),
		3*w/4, h*60/100)

	// Wind
	drawCentered(img, medFont, windString(wx), w/2, h*70/100)

	// Rain rate — only when non-zero
	if wx.Current.RainRate.Value > 0 {
		drawCentered(img, medFont,
			fmt.Sprintf("Rain  %.2f in/h", wx.Current.RainRate.Value),
			w/2, h*80/100)
	}

	// Footer
	drawHLine(img, h*910/1000, margin, w-margin)
	drawCentered(img, smallFont, "Updated  "+wx.Generation.Time, w/2, h*960/1000)

	return img, nil
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

func compassDir(deg float64) string {
	dirs := [16]string{"N", "NNE", "NE", "ENE", "E", "ESE", "SE", "SSE", "S", "SSW", "SW", "WSW", "W", "WNW", "NW", "NNW"}
	return dirs[int(math.Round(deg/22.5))%16]
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
