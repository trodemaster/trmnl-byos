#!/usr/bin/env python3
# Recolor a Meteocons "fill" SVG for bold grayscale rasterization. See README.md.
#
# The source icons use a very light UI palette (e.g. #F3F7FE -> #E6EFFC blue
# gradients) designed to sit subtly on a white background. Converting that
# straight to grayscale (or stretching it after rasterizing) produces a
# washed-out icon or banding artifacts, since the source range is only ~15
# gray levels wide to begin with. Instead this remaps each SVG's *own* colors
# (by rank, lightest to darkest) onto a fixed, much bolder gray palette
# *before* rasterization, so librsvg computes the anti-aliased gradient at
# full vector precision -- no banding, real contrast.
import re
import sys

path = sys.argv[1]
out = sys.argv[2]
with open(path) as f:
    svg = f.read()

hexes = sorted(set(re.findall(r'#[0-9A-Fa-f]{6}\b', svg)))


def luminance(hexcolor):
    r = int(hexcolor[1:3], 16) / 255
    g = int(hexcolor[3:5], 16) / 255
    b = int(hexcolor[5:7], 16) / 255
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


ranked = sorted(hexes, key=luminance, reverse=True)
n = len(ranked)
if n == 1:
    targets = [140]
else:
    light, dark = 205, 70
    targets = [round(light + (dark - light) * i / (n - 1)) for i in range(n)]

mapping = {color: f'#{gray:02x}{gray:02x}{gray:02x}' for color, gray in zip(ranked, targets)}
for old, new in mapping.items():
    svg = svg.replace(old, new)

with open(out, 'w') as f:
    f.write(svg)

print(path, '->', mapping)
