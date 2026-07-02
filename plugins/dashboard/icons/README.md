# Weather icons

Rasterized from [Meteocons](https://github.com/basmilius/weather-icons)
(`@meteocons/svg-static`, MIT licensed), `fill` style — gradient-shaded SVGs
converted to grayscale, so the shading survives as real tonal depth instead
of flattening to a single gray.

| File | Meteocons source (`fill/<name>.svg`) | wmoCategory |
|---|---|---|
| `sun.png` | `clear-day` | `sun` |
| `partly.png` | `partly-cloudy-day` | `partly` |
| `cloud.png` | `cloudy` | `cloud` |
| `fog.png` | `fog` (not `fog-day` — that one renders as a sunrise) | `fog` |
| `drizzle.png` | `drizzle` | `drizzle` |
| `rain.png` | `rain` | `rain` |
| `snow.png` | `snow` | `snow` |
| `thunder.png` | `thunderstorms-rain` | `thunder` |

## Regenerating

The source SVGs use a very light UI palette (e.g. `#F3F7FE` -> `#E6EFFC` blue
gradients) meant to sit subtly on a white background. Rasterizing that
straight to grayscale produces a washed-out icon (the real content is
squeezed into roughly gray 237-254, a ~15-level-wide band) — and stretching
that narrow band's contrast *after* rasterizing causes visible banding, since
there aren't enough source gray levels to stretch smoothly. `recolor.py`
fixes this the right way: it remaps each SVG's own colors, by rank (lightest
to darkest), onto a fixed bold gray palette (205 -> 70) *before*
rasterization, so librsvg computes the anti-aliased gradient at full vector
precision. No banding, real contrast.

```bash
curl -s https://registry.npmjs.org/@meteocons/svg-static/latest \
  | python3 -c "import json,sys; print(json.load(sys.stdin)['dist']['tarball'])" \
  | xargs curl -sL -o /tmp/meteocons.tgz
mkdir -p /tmp/meteocons-pkg && tar -xzf /tmp/meteocons.tgz -C /tmp/meteocons-pkg --strip-components=1

python3 recolor.py /tmp/meteocons-pkg/fill/clear-day.svg /tmp/sun-recolored.svg
rsvg-convert -w 256 -h 256 /tmp/sun-recolored.svg -o /tmp/sun.png
convert /tmp/sun.png -colorspace Gray sun.png
# ...repeat per file in the table above. Use rsvg-convert, not ImageMagick's
# built-in SVG delegate — it silently drops some elements (e.g. clear-day's
# sun disc outline circle never renders). Verify the result preserved alpha
# with `identify -verbose sun.png` (look for "Type: GrayscaleAlpha").
```
