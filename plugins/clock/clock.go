package clock

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"log"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/trodemaster/trmnl-byos/internal/device"
	"github.com/trodemaster/trmnl-byos/internal/plugin"
)

var (
	largeFont font.Face
	smallFont font.Face
)

func init() {
	tt, err := opentype.Parse(goregular.TTF)
	if err != nil {
		log.Fatalf("clock: parse font: %v", err)
	}
	const dpi = 96
	largeFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 120, DPI: dpi})
	smallFont, _ = opentype.NewFace(tt, &opentype.FaceOptions{Size: 36, DPI: dpi})
	plugin.Register(&clockPlugin{})
}

type clockPlugin struct{}

func (c *clockPlugin) Name() string { return "clock" }

func (c *clockPlugin) Render(_ context.Context, d *device.Device) (*image.Gray, error) {
	w, h := 800, 480
	if d.Width > 0 {
		w = d.Width
	}
	if d.Height > 0 {
		h = d.Height
	}

	img := image.NewGray(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	now := time.Now()
	drawCentered(img, largeFont, now.Format("15:04"), w/2, h/2-20)
	drawCentered(img, smallFont, now.Format("Monday, January 2 2006"), w/2, h/2+80)

	return img, nil
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
