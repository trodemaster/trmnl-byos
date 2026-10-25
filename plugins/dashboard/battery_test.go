package dashboard

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"github.com/trodemaster/trmnl-byos/internal/device"
)

func TestHasBattery(t *testing.T) {
	cases := []struct {
		name string
		d    device.Device
		want bool
	}{
		{"never reported", device.Device{}, false},
		{"percent", device.Device{BatteryPercent: 80}, true},
		{"voltage only", device.Device{BatteryVoltage: 3.9}, true},
		{"charging still shown", device.Device{BatteryPercent: 10, Charging: true}, true},
	}
	for _, c := range cases {
		if got := hasBattery(&c.d); got != c.want {
			t.Errorf("%s: hasBattery = %v, want %v", c.name, got, c.want)
		}
	}
}

// The fill must track the charge: a fuller battery has more black pixels.
func TestDrawBatteryFillTracksCharge(t *testing.T) {
	black := func(pct float64) int {
		img := image.NewGray(image.Rect(0, 0, 400, 100))
		draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
		drawBattery(img, smallFont, 380, 50, &device.Device{BatteryPercent: pct})
		n := 0
		for _, p := range img.Pix {
			if p == 0 {
				n++
			}
		}
		return n
	}
	if lo, hi := black(10), black(90); lo >= hi {
		t.Errorf("fill at 10%% (%d px) should be smaller than at 90%% (%d px)", lo, hi)
	}
}

func TestOnPower(t *testing.T) {
	cases := []struct {
		d    device.Device
		want bool
	}{
		{device.Device{}, false},
		{device.Device{Charging: true}, true},
		{device.Device{USBConnected: true}, true}, // plugged in and full
	}
	for i, c := range cases {
		if got := onPower(&c.d); got != c.want {
			t.Errorf("case %d: onPower = %v, want %v", i, got, c.want)
		}
	}
}

func TestBoltDrawsOnlyOnPower(t *testing.T) {
	black := func(d device.Device) int {
		img := image.NewGray(image.Rect(0, 0, 400, 100))
		draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
		d.BatteryPercent = 50
		drawBattery(img, smallFont, 380, 50, &d)
		n := 0
		for _, p := range img.Pix {
			if p == 0 {
				n++
			}
		}
		return n
	}
	if off, on := black(device.Device{}), black(device.Device{USBConnected: true}); on <= off {
		t.Errorf("bolt should add pixels: off=%d on=%d", off, on)
	}
}
