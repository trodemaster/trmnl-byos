package forecast

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

const forecastURL = "https://api.open-meteo.com/v1/forecast" +
	"?latitude=47.6062&longitude=-122.3321" +
	"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max" +
	"&temperature_unit=fahrenheit&wind_speed_unit=mph" +
	"&precipitation_unit=inch&timezone=America%2FLos_Angeles" +
	"&forecast_days=5"

type forecastResponse struct {
	Daily struct {
		Time                 []string  `json:"time"`
		WeatherCode          []int     `json:"weather_code"`
		TempMax              []float64 `json:"temperature_2m_max"`
		TempMin              []float64 `json:"temperature_2m_min"`
		PrecipProbabilityMax []int     `json:"precipitation_probability_max"`
	} `json:"daily"`
}

var (
	dayFont   font.Face // day name
	tempFont  font.Face // high / low temperatures
	labelFont font.Face // condition text, precip probability
	hdrFont   font.Face // header line
)

func init() {
	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		log.Fatalf("forecast: parse font: %v", err)
	}
	const dpi = 96
	dayFont, _   = opentype.NewFace(tt, &opentype.FaceOptions{Size: 72, DPI: dpi})
	tempFont, _  = opentype.NewFace(tt, &opentype.FaceOptions{Size: 80, DPI: dpi})
	labelFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 40, DPI: dpi})
	hdrFont, _   = opentype.NewFace(tt, &opentype.FaceOptions{Size: 36, DPI: dpi})
	plugin.Register(&forecastPlugin{})
}

type forecastPlugin struct{}

func (p *forecastPlugin) Name() string { return "forecast" }

func (p *forecastPlugin) Render(_ context.Context, d *device.Device) (*image.Gray, error) {
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

	fc, err := fetchForecast()
	if err != nil {
		drawCentered(img, dayFont, "Forecast unavailable", w/2, h/2-60)
		drawCentered(img, labelFont, err.Error(), w/2, h/2+60)
		return img, nil
	}

	n := len(fc.Daily.Time)
	if n > 5 {
		n = 5
	}
	if n == 0 {
		drawCentered(img, dayFont, "No forecast data", w/2, h/2)
		return img, nil
	}

	// Header
	drawLeft(img, hdrFont, "5-Day Forecast  ·  Seattle, WA", margin, h*65/1000)
	drawRight(img, hdrFont, time.Now().Format("Mon Jan 2  3:04 PM"), w-margin, h*65/1000)
	drawHLine(img, h*90/1000, margin, w-margin)

	colW := (w - 2*margin) / n
	iconR := colW * 40 / 100

	for i := 0; i < n; i++ {
		cx := margin + colW*i + colW/2

		// Day name
		t, _ := time.Parse("2006-01-02", fc.Daily.Time[i])
		dayName := t.Format("Mon")
		if i == 0 {
			dayName = "Today"
		}
		drawCentered(img, dayFont, dayName, cx, h*200/1000)

		// Weather icon
		drawWeatherIcon(img, cx, h*415/1000, iconR, fc.Daily.WeatherCode[i])

		// Condition label
		drawCentered(img, labelFont, wmoDescription(fc.Daily.WeatherCode[i]), cx, h*620/1000)

		// High / low temps
		if i < len(fc.Daily.TempMax) {
			drawCentered(img, tempFont, fmt.Sprintf("H %.0f°", fc.Daily.TempMax[i]), cx, h*745/1000)
		}
		if i < len(fc.Daily.TempMin) {
			drawCentered(img, tempFont, fmt.Sprintf("L %.0f°", fc.Daily.TempMin[i]), cx, h*870/1000)
		}

		// Precip probability
		if i < len(fc.Daily.PrecipProbabilityMax) {
			drawCentered(img, labelFont,
				fmt.Sprintf("%d%% rain", fc.Daily.PrecipProbabilityMax[i]),
				cx, h*960/1000)
		}

		// Vertical divider (not after last column)
		if i < n-1 {
			x := margin + colW*(i+1)
			for y := h * 100 / 1000; y < h*980/1000; y++ {
				img.SetGray(x, y, color.Gray{0})
			}
		}
	}

	return img, nil
}

func fetchForecast() (*forecastResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(forecastURL)
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

// ── WMO helpers ─────────────────────────────────────────────────────────────

func wmoDescription(code int) string {
	switch code {
	case 0:
		return "Clear"
	case 1:
		return "Mostly Clear"
	case 2:
		return "Partly Cloudy"
	case 3:
		return "Overcast"
	case 45, 48:
		return "Fog"
	case 51, 53:
		return "Drizzle"
	case 55, 56, 57:
		return "Heavy Drizzle"
	case 61, 63:
		return "Rain"
	case 65, 66, 67:
		return "Heavy Rain"
	case 71, 73:
		return "Snow"
	case 75, 77:
		return "Heavy Snow"
	case 80, 81:
		return "Showers"
	case 82:
		return "Heavy Showers"
	case 85, 86:
		return "Snow Showers"
	case 95:
		return "Thunderstorm"
	case 96, 99:
		return "T-Storm + Hail"
	default:
		return fmt.Sprintf("Code %d", code)
	}
}

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

// ── Icon drawing ─────────────────────────────────────────────────────────────

func drawWeatherIcon(img *image.Gray, cx, cy, r, code int) {
	switch wmoCategory(code) {
	case "sun":
		iconSun(img, cx, cy, r)
	case "partly":
		iconPartly(img, cx, cy, r)
	case "cloud":
		iconCloud(img, cx, cy, r)
	case "fog":
		iconFog(img, cx, cy, r)
	case "drizzle":
		iconRain(img, cx, cy, r, true)
	case "rain":
		iconRain(img, cx, cy, r, false)
	case "snow":
		iconSnow(img, cx, cy, r)
	default: // thunder
		iconThunder(img, cx, cy, r)
	}
}

func iconSun(img *image.Gray, cx, cy, r int) {
	lw := max(r/18, 5)
	sunR := r * 42 / 100
	strokeCircle(img, cx, cy, sunR, lw)
	rayIn := sunR + lw + r*5/100
	rayOut := r * 92 / 100
	for i := 0; i < 8; i++ {
		ang := float64(i) * math.Pi / 4
		drawLine(img,
			cx+int(float64(rayIn)*math.Cos(ang)),
			cy+int(float64(rayIn)*math.Sin(ang)),
			cx+int(float64(rayOut)*math.Cos(ang)),
			cy+int(float64(rayOut)*math.Sin(ang)),
			lw)
	}
}

func iconCloud(img *image.Gray, cx, cy, r int) {
	cloudSilhouette(img, cx, cy, r)
}

func iconPartly(img *image.Gray, cx, cy, r int) {
	// Sun outline behind and to the upper-right
	lw := max(r/18, 5)
	sCX := cx + r*18/100
	sCY := cy - r*18/100
	sR := r * 38 / 100
	strokeCircle(img, sCX, sCY, sR, lw)
	rayIn := sR + lw + r*4/100
	rayOut := r * 76 / 100
	for i := 0; i < 8; i++ {
		ang := float64(i) * math.Pi / 4
		drawLine(img,
			sCX+int(float64(rayIn)*math.Cos(ang)),
			sCY+int(float64(rayIn)*math.Sin(ang)),
			sCX+int(float64(rayOut)*math.Cos(ang)),
			sCY+int(float64(rayOut)*math.Sin(ang)),
			lw)
	}
	// Cloud silhouette in front, lower-left — paints over part of the sun
	cloudSilhouette(img, cx-r*10/100, cy+r*14/100, r*72/100)
}

func iconFog(img *image.Gray, cx, cy, r int) {
	lw := max(r/13, 7)
	halves := []int{r * 85 / 100, r * 70 / 100, r * 55 / 100, r * 38 / 100}
	for i, hw := range halves {
		y := cy - r*28/100 + i*r*22/100
		drawLine(img, cx-hw+lw, y, cx+hw-lw, y, lw*2)
	}
}

func iconRain(img *image.Gray, cx, cy, r int, light bool) {
	cloudSilhouette(img, cx, cy-r*12/100, r*80/100)
	lw := max(r/20, 4)
	nDrops := 3
	if light {
		nDrops = 2
	}
	spacing := r * 24 / 100
	startX := cx - (nDrops-1)*spacing/2
	dropLen := r * 18 / 100
	for i := 0; i < nDrops; i++ {
		x := startX + i*spacing
		y := cy + r*42/100
		drawLine(img, x, y, x+r*4/100, y+dropLen, lw)
	}
}

func iconSnow(img *image.Gray, cx, cy, r int) {
	cloudSilhouette(img, cx, cy-r*12/100, r*80/100)
	lw := max(r/20, 4)
	flakeR := r * 14 / 100
	nFlakes := 3
	spacing := r * 26 / 100
	startX := cx - (nFlakes-1)*spacing/2
	for i := 0; i < nFlakes; i++ {
		fx := startX + i*spacing
		fy := cy + r*52/100
		for j := 0; j < 3; j++ {
			ang := float64(j) * math.Pi / 3
			drawLine(img,
				fx+int(float64(flakeR)*math.Cos(ang)),
				fy+int(float64(flakeR)*math.Sin(ang)),
				fx-int(float64(flakeR)*math.Cos(ang)),
				fy-int(float64(flakeR)*math.Sin(ang)),
				lw)
		}
	}
}

func iconThunder(img *image.Gray, cx, cy, r int) {
	cloudSilhouette(img, cx, cy-r*18/100, r*76/100)
	lw := max(r/13, 6)
	// Lightning bolt: 3-segment zigzag
	pts := [][2]int{
		{cx + r*8/100, cy + r*22/100},
		{cx - r*12/100, cy + r*52/100},
		{cx + r*5/100, cy + r*47/100},
		{cx - r*14/100, cy + r*80/100},
	}
	for i := range pts[:len(pts)-1] {
		drawLine(img, pts[i][0], pts[i][1], pts[i+1][0], pts[i+1][1], lw)
	}
}

// cloudSilhouette draws 3 overlapping filled circles forming a cloud silhouette.
func cloudSilhouette(img *image.Gray, cx, cy, r int) {
	fillCircle(img, cx, cy+r*18/100, r*55/100)
	fillCircle(img, cx-r*32/100, cy-r*5/100, r*36/100)
	fillCircle(img, cx+r*22/100, cy-r*18/100, r*44/100)
}

// ── Pixel drawing primitives ──────────────────────────────────────────────────

var ink = color.Gray{Y: 0}

func fillCircle(img *image.Gray, cx, cy, r int) {
	r2 := r * r
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r2 {
				img.SetGray(x, y, ink)
			}
		}
	}
}

func strokeCircle(img *image.Gray, cx, cy, r, lw int) {
	inner2 := r * r
	outer := r + lw
	outer2 := outer * outer
	for y := cy - outer; y <= cy+outer; y++ {
		for x := cx - outer; x <= cx+outer; x++ {
			d2 := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			if d2 >= inner2 && d2 <= outer2 {
				img.SetGray(x, y, ink)
			}
		}
	}
}

func drawLine(img *image.Gray, x0, y0, x1, y1, lw int) {
	dx := float64(x1 - x0)
	dy := float64(y1 - y0)
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}
	steps := int(length*2) + 1
	hw := lw / 2
	hw2 := hw * hw
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px := int(math.Round(float64(x0) + dx*t))
		py := int(math.Round(float64(y0) + dy*t))
		for ky := -hw; ky <= hw; ky++ {
			for kx := -hw; kx <= hw; kx++ {
				if kx*kx+ky*ky <= hw2 {
					img.SetGray(px+kx, py+ky, ink)
				}
			}
		}
	}
}

// ── Text helpers ──────────────────────────────────────────────────────────────

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

func drawHLine(img *image.Gray, y, x0, x1 int) {
	for x := x0; x <= x1; x++ {
		img.SetGray(x, y, ink)
	}
}
