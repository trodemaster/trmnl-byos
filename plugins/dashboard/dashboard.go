package dashboard

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

const (
	wxURL = "http://wx.jibb.tv/weewx.json"
	fcURL = "https://api.open-meteo.com/v1/forecast" +
		"?latitude=47.6062&longitude=-122.3321" +
		"&daily=weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max" +
		"&temperature_unit=fahrenheit&wind_speed_unit=mph" +
		"&precipitation_unit=inch&timezone=America%2FLos_Angeles" +
		"&forecast_days=5"
)

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

// ── Fonts ────────────────────────────────────────────────────────────────────

var (
	heroFont  font.Face // current temperature
	medFont   font.Face // current conditions data rows + forecast day names
	smallFont font.Face // header text, precip labels
)

func init() {
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
	divY := h * 514 / 1000 // ≈722px at 1404 height
	fcH := h - divY        // forecast section height ≈682px

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
			w/2, h*235/1000)

		drawLeft(img, medFont,
			fmt.Sprintf("Humidity  %.0f%%", wx.Current.Humidity.Value),
			margin, h*338/1000)
		drawRight(img, medFont,
			fmt.Sprintf("%.2f inHg", wx.Current.Barometer.Value),
			w-margin, h*338/1000)

		drawCentered(img, medFont, windString(wx), w/2, h*405/1000)

		drawLeft(img, medFont,
			fmt.Sprintf("High  %.0f%s", wx.Day.MaxTemp.Value, wx.Day.MaxTemp.Units),
			margin, h*467/1000)
		drawRight(img, medFont,
			fmt.Sprintf("Low  %.0f%s", wx.Day.MinTemp.Value, wx.Day.MinTemp.Units),
			w-margin, h*467/1000)

		if wx.Day.RainTotal.Value > 0 {
			rain := fmt.Sprintf("Rain  %.2f in today", wx.Day.RainTotal.Value)
			if wx.Current.RainRate.Value > 0 {
				rain += fmt.Sprintf("  (%.2f in/h)", wx.Current.RainRate.Value)
			}
			drawCentered(img, smallFont, rain, w/2, h*490/1000)
		}
	}

	// Section divider (3px thick)
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

		drawLeft(img, smallFont, "5-Day Forecast", margin, divY+fcH*48/1000)
		drawHLine(img, divY+fcH*78/1000, margin, w-margin)

		colW := (w - 2*margin) / n
		iconR := colW * 24 / 100

		for i := 0; i < n; i++ {
			cx := margin + colW*i + colW/2

			t, _ := time.Parse("2006-01-02", fc.Daily.Time[i])
			dayName := t.Format("Mon")
			if i == 0 {
				dayName = "Today"
			}
			drawCentered(img, medFont, dayName, cx, divY+fcH*210/1000)

			if i < len(fc.Daily.WeatherCode) {
				drawWeatherIcon(img, cx, divY+fcH*395/1000, iconR, fc.Daily.WeatherCode[i])
			}

			if i < len(fc.Daily.TempMax) {
				drawCentered(img, medFont,
					fmt.Sprintf("H %.0f°", fc.Daily.TempMax[i]),
					cx, divY+fcH*635/1000)
			}
			if i < len(fc.Daily.TempMin) {
				drawCentered(img, medFont,
					fmt.Sprintf("L %.0f°", fc.Daily.TempMin[i]),
					cx, divY+fcH*790/1000)
			}
			if i < len(fc.Daily.PrecipProbabilityMax) {
				drawCentered(img, smallFont,
					fmt.Sprintf("%d%% rain", fc.Daily.PrecipProbabilityMax[i]),
					cx, divY+fcH*935/1000)
			}

			if i < n-1 {
				x := margin + colW*(i+1)
				for y := divY + fcH*85/1000; y < h*980/1000; y++ {
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
	default:
		iconThunder(img, cx, cy, r)
	}
}

func iconSun(img *image.Gray, cx, cy, r int) {
	lw := max(r/18, 4)
	sunR := r * 42 / 100
	strokeCircle(img, cx, cy, sunR, lw)
	rayIn := sunR + lw + r*5/100
	rayOut := r * 92 / 100
	for i := 0; i < 8; i++ {
		ang := float64(i) * math.Pi / 4
		drawLine(img,
			cx+int(float64(rayIn)*math.Cos(ang)), cy+int(float64(rayIn)*math.Sin(ang)),
			cx+int(float64(rayOut)*math.Cos(ang)), cy+int(float64(rayOut)*math.Sin(ang)),
			lw)
	}
}

func iconCloud(img *image.Gray, cx, cy, r int) {
	cloudSilhouette(img, cx, cy, r)
}

func iconPartly(img *image.Gray, cx, cy, r int) {
	lw := max(r/18, 4)
	sCX, sCY := cx+r*18/100, cy-r*18/100
	sR := r * 38 / 100
	strokeCircle(img, sCX, sCY, sR, lw)
	rayIn := sR + lw + r*4/100
	rayOut := r * 76 / 100
	for i := 0; i < 8; i++ {
		ang := float64(i) * math.Pi / 4
		drawLine(img,
			sCX+int(float64(rayIn)*math.Cos(ang)), sCY+int(float64(rayIn)*math.Sin(ang)),
			sCX+int(float64(rayOut)*math.Cos(ang)), sCY+int(float64(rayOut)*math.Sin(ang)),
			lw)
	}
	cloudSilhouette(img, cx-r*10/100, cy+r*14/100, r*72/100)
}

func iconFog(img *image.Gray, cx, cy, r int) {
	lw := max(r/13, 5)
	halves := []int{r * 85 / 100, r * 70 / 100, r * 55 / 100, r * 38 / 100}
	for i, hw := range halves {
		y := cy - r*28/100 + i*r*22/100
		drawLine(img, cx-hw+lw, y, cx+hw-lw, y, lw*2)
	}
}

func iconRain(img *image.Gray, cx, cy, r int, light bool) {
	cloudSilhouette(img, cx, cy-r*12/100, r*80/100)
	lw := max(r/20, 3)
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
	lw := max(r/20, 3)
	flakeR := r * 14 / 100
	spacing := r * 26 / 100
	startX := cx - spacing
	for i := 0; i < 3; i++ {
		fx := startX + i*spacing
		fy := cy + r*52/100
		for j := 0; j < 3; j++ {
			ang := float64(j) * math.Pi / 3
			drawLine(img,
				fx+int(float64(flakeR)*math.Cos(ang)), fy+int(float64(flakeR)*math.Sin(ang)),
				fx-int(float64(flakeR)*math.Cos(ang)), fy-int(float64(flakeR)*math.Sin(ang)),
				lw)
		}
	}
}

func iconThunder(img *image.Gray, cx, cy, r int) {
	cloudSilhouette(img, cx, cy-r*18/100, r*76/100)
	lw := max(r/13, 5)
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

func cloudSilhouette(img *image.Gray, cx, cy, r int) {
	fillCircle(img, cx, cy+r*18/100, r*55/100)
	fillCircle(img, cx-r*32/100, cy-r*5/100, r*36/100)
	fillCircle(img, cx+r*22/100, cy-r*18/100, r*44/100)
}

// ── Pixel primitives ──────────────────────────────────────────────────────────

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
		img.SetGray(x, y, ink)
	}
}
