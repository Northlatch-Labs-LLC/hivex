#!/usr/bin/env python3
"""Generate the Hivex logo kit (SVG + README) into the hivex-logo folder.

Concept: a honeycomb cell (hexagon) acts as the *harness*; the bold X through
the middle is the orchestration spine; six satellite nodes are the agents,
wired to one glowing core.
"""
import math
import os

BASE = os.path.dirname(os.path.abspath(__file__))
CX = CY = 256.0
INK = "#161513"

POINTY = [-90, -30, 30, 90, 150, 210]   # pointy-top hexagon vertices
FLAT = [0, 60, 120, 180, 240, 300]      # flat-top hexagon (faint backdrop)
R_HEX, R_NODE = 148, 118


def pt(r, deg):
    a = math.radians(deg)
    return (CX + r * math.cos(a), CY + r * math.sin(a))


def f(x):
    s = f"{x:.2f}".rstrip("0").rstrip(".")
    return s if s else "0"


def hex_path(r, degs):
    pts = [pt(r, d) for d in degs]
    d = f"M{f(pts[0][0])} {f(pts[0][1])}"
    for x, y in pts[1:]:
        d += f" L{f(x)} {f(y)}"
    return d + " Z"


def line(a, b, **kw):
    return line, a, b, kw


def seg(a, b):
    return f"M{f(a[0])} {f(a[1])} L{f(b[0])} {f(b[1])}"


def lerp(a, b, t):
    return (a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t)


# ---------------------------------------------------------------- palettes
DARK = dict(
    mono=False,
    hex_a="#FFD44F", hex_b="#FF8A1E",
    x_a="#FFDF7E", x_b="#FF9330",
    hex_glow="#FF9E2E", hex_glow_op=0.10,
    x_glow="#FFAE3D", x_glow_op=0.12,
    spine="#FFC168", spine_op=0.50,
    node_ring="#FFC168",
    halo="#FFBE4D", halo_op=0.40,
    hub_ring="#FFD98A", hub_inner="#FFF3D8",
    pulse="#FFF0CB", pulse_op=0.95,
    faint="#FFB556", faint_op=0.07,
)

LIGHT = dict(
    mono=False,
    hex_a="#D97F0F", hex_b="#9C5606",
    x_a="#E8940F", x_b="#A65E07",
    hex_glow="#C97A1A", hex_glow_op=0.08,
    x_glow="#C97A1A", x_glow_op=0.10,
    spine="#B97A2A", spine_op=0.55,
    node_ring="#B97A2A",
    halo="#E39A3B", halo_op=0.15,
    hub_ring="#C98B2E", hub_inner="#FBEFD4",
    pulse="#A66A12", pulse_op=0.90,
    faint="#C98B2E", faint_op=0.10,
)

MONO = dict(
    mono=True,
    hex_a=INK, hex_b=INK, x_a=INK, x_b=INK,
    hex_glow=None, hex_glow_op=0,
    x_glow=None, x_glow_op=0,
    spine=INK, spine_op=0.40,
    node_ring=INK,
    halo=None, halo_op=0,
    hub_ring=INK, hub_inner="#FFFFFF",
    pulse=INK, pulse_op=0.80,
    faint=INK, faint_op=0.08,
)

TILE_BG_A, TILE_BG_B = "#1A2434", "#0A0F18"


# ---------------------------------------------------------------- builders
def mark_defs(p):
    if p["mono"]:
        return ""
    return f"""  <linearGradient id="gHex" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="{p['hex_a']}"/>
    <stop offset="1" stop-color="{p['hex_b']}"/>
  </linearGradient>
  <linearGradient id="gX" x1="0" y1="0" x2="1" y2="1">
    <stop offset="0" stop-color="{p['x_a']}"/>
    <stop offset="1" stop-color="{p['x_b']}"/>
  </linearGradient>
  <radialGradient id="gHalo">
    <stop offset="0" stop-color="{p['halo']}" stop-opacity="{p['halo_op']}"/>
    <stop offset="1" stop-color="{p['halo']}" stop-opacity="0"/>
  </radialGradient>
"""


def mark_body(p):
    hex_stroke = INK if p["mono"] else "url(#gHex)"
    x_stroke = INK if p["mono"] else "url(#gX)"
    node_fill = INK if p["mono"] else "url(#gHex)"
    hub_core = INK if p["mono"] else "url(#gX)"
    b = []

    # faint flat-top hexagon backdrop (depth)
    b.append(f'<path d="{hex_path(205, FLAT)}" fill="none" stroke="{p["faint"]}" '
             f'stroke-opacity="{p["faint_op"]}" stroke-width="3"/>')

    # harness hexagon: soft under-glow + main stroke
    if p["hex_glow"]:
        b.append(f'<path d="{hex_path(R_HEX, POINTY)}" fill="none" stroke="{p["hex_glow"]}" '
                 f'stroke-opacity="{p["hex_glow_op"]}" stroke-width="30" stroke-linejoin="round"/>')
    b.append(f'<path d="{hex_path(R_HEX, POINTY)}" fill="none" stroke="{hex_stroke}" '
             f'stroke-width="14" stroke-linejoin="round"/>')

    # vertical scheduler spine (thin, secondary)
    b.append(f'<path d="{seg(pt(R_NODE, -90), pt(R_NODE, 90))}" stroke="{p["spine"]}" '
             f'stroke-opacity="{p["spine_op"]}" stroke-width="9" stroke-linecap="round"/>')

    # the X: orchestration spine (bold, primary)
    if p["x_glow"]:
        b.append(f'<path d="{seg(pt(R_NODE, 210), pt(R_NODE, 30))} {seg(pt(R_NODE, 330), pt(R_NODE, 150))}" '
                 f'stroke="{p["x_glow"]}" stroke-opacity="{p["x_glow_op"]}" stroke-width="36" '
                 f'stroke-linecap="round" fill="none"/>')
    b.append(f'<path d="{seg(pt(R_NODE, 210), pt(R_NODE, 30))} {seg(pt(R_NODE, 330), pt(R_NODE, 150))}" '
             f'stroke="{x_stroke}" stroke-width="18" stroke-linecap="round" fill="none"/>')

    # agent nodes: filled on the X plane, hollow on the vertical axis
    for d in (30, 150, 210, 330):
        x, y = pt(R_NODE, d)
        b.append(f'<circle cx="{f(x)}" cy="{f(y)}" r="15" fill="{node_fill}"/>')
    for d in (-90, 90):
        x, y = pt(R_NODE, d)
        b.append(f'<circle cx="{f(x)}" cy="{f(y)}" r="11" fill="none" stroke="{p["node_ring"]}" '
                 f'stroke-width="5"/>')

    # task pulses travelling the X arms
    for d in (30, 150, 210, 330):
        x, y = lerp((CX, CY), pt(R_NODE, d), 0.55)
        b.append(f'<circle cx="{f(x)}" cy="{f(y)}" r="5" fill="{p["pulse"]}" '
                 f'stroke="none" opacity="{p["pulse_op"]}"/>')

    # orchestrator core
    if not p["mono"]:
        b.append('<circle cx="256" cy="256" r="70" fill="url(#gHalo)"/>')
    b.append(f'<circle cx="256" cy="256" r="32" fill="none" stroke="{p["hub_ring"]}" stroke-width="5"/>')
    b.append(f'<circle cx="256" cy="256" r="18" fill="{hub_core}"/>')
    b.append(f'<circle cx="256" cy="256" r="8.5" fill="{p["hub_inner"]}"/>')

    return "\n  ".join(b)


def tile_defs():
    return f"""  <linearGradient id="gBg" x1="0" y1="0" x2="0" y2="1">
    <stop offset="0" stop-color="{TILE_BG_A}"/>
    <stop offset="1" stop-color="{TILE_BG_B}"/>
  </linearGradient>
  <radialGradient id="gSheen" cx="0.5" cy="0.18" r="0.75">
    <stop offset="0" stop-color="#FFB556" stop-opacity="0.07"/>
    <stop offset="1" stop-color="#FFB556" stop-opacity="0"/>
  </radialGradient>
"""


def tile_body():
    return f"""  <rect width="512" height="512" rx="116" fill="url(#gBg)"/>
  <rect x="1" y="1" width="510" height="510" rx="115" fill="none"
        stroke="#FFFFFF" stroke-opacity="0.06" stroke-width="2"/>
  <ellipse cx="256" cy="80" rx="280" ry="150" fill="url(#gSheen)"/>"""


def svg_doc(w, h, defs, body, title):
    return (f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" '
            f'viewBox="0 0 {w} {h}">\n<title>{title}</title>\n<defs>\n{defs}</defs>\n'
            f'{body}\n</svg>\n')


def write(name, content):
    path = os.path.join(BASE, name)
    with open(path, "w") as fh:
        fh.write(content)
    print("wrote", path)


# ---------------------------------------------------------------- letters
# Stroke-built geometric caps (cap height 100, stroke 16, round terminals)
# so the wordmark needs no font. Widths: H60 I14 V64 E54 X64, gaps 26.
LETTERS = {
    "H": lambda dx: f"M{dx+8} 8 V92 M{dx+52} 8 V92 M{dx+8} 50 H{dx+52}",
    "I": lambda dx: f"M{dx+7} 8 V92",
    "V": lambda dx: f"M{dx+8} 8 L{dx+32} 92 L{dx+56} 8",
    "E": lambda dx: f"M{dx+8} 8 V92 M{dx+8} 8 H{dx+46} M{dx+8} 50 H{dx+38} M{dx+8} 92 H{dx+46}",
    "X": lambda dx: f"M{dx+8} 8 L{dx+56} 92 M{dx+56} 8 L{dx+8} 92",
}
ADV = {"H": 60, "I": 14, "V": 64, "E": 54, "X": 64}
GAP = 26


def wordmark_d():
    dx = 0
    parts = []
    for i, ch in enumerate("HIVEX"):
        parts.append(LETTERS[ch](dx))
        dx += ADV[ch] + (GAP if i < 4 else 0)
    return " ".join(parts)


WORDMARK_W = sum(ADV.values()) + GAP * 4  # 360


def wordmark_group(x, y, s, color):
    return (f'<g transform="translate({f(x)} {f(y)}) scale({s})">'
            f'<path d="{wordmark_d()}" fill="none" stroke="{color}" stroke-width="16" '
            f'stroke-linecap="round" stroke-linejoin="round"/></g>')


def tagline(x, y, size, color, length):
    return (f'<text x="{f(x)}" y="{f(y)}" font-family="\'Avenir Next\',\'Helvetica Neue\','
            f'Helvetica,Arial,sans-serif" font-size="{size}" font-weight="600" '
            f'letter-spacing="{size*0.18:.1f}" fill="{color}" '
            f'textLength="{f(length)}" lengthAdjust="spacing">AGENTIC ORCHESTRATION HARNESS</text>')


# ---------------------------------------------------------------- artifacts
def build():
    title = "Hivex — Agentic Orchestration Harness"

    # 1. primary app icon (dark tile)
    write("hivex-icon.svg", svg_doc(
        512, 512, mark_defs(DARK) + tile_defs(),
        tile_body() + "\n  " + mark_body(DARK), title))

    # 2. simplified glyph for tiny sizes (hexagon + X + core only)
    glyph = f"""  {tile_body()}
  <path d="{hex_path(152, POINTY)}" fill="none" stroke="url(#gHex)" stroke-width="20" stroke-linejoin="round"/>
  <path d="{seg(pt(110, 210), pt(110, 30))} {seg(pt(110, 330), pt(110, 150))}"
        stroke="url(#gX)" stroke-width="26" stroke-linecap="round" fill="none"/>
  <circle cx="256" cy="256" r="22" fill="#FFEDC2"/>"""
    write("hivex-glyph.svg", svg_doc(
        512, 512,
        '<linearGradient id="gHex" x1="0" y1="0" x2="1" y2="1">'
        f'<stop offset="0" stop-color="{DARK["hex_a"]}"/><stop offset="1" stop-color="{DARK["hex_b"]}"/>'
        '</linearGradient>'
        '<linearGradient id="gX" x1="0" y1="0" x2="1" y2="1">'
        f'<stop offset="0" stop-color="{DARK["x_a"]}"/><stop offset="1" stop-color="{DARK["x_b"]}"/>'
        '</linearGradient>' + tile_defs(), glyph, "Hivex glyph"))

    # 3. mark-only variants
    write("hivex-mark.svg", svg_doc(512, 512, mark_defs(DARK), "  " + mark_body(DARK), title))
    write("hivex-mark-light.svg", svg_doc(512, 512, mark_defs(LIGHT), "  " + mark_body(LIGHT), title))
    write("hivex-mono.svg", svg_doc(512, 512, mark_defs(MONO), "  " + mark_body(MONO), title))

    # 4. horizontal lockups
    for name, p, word, tag in (
        ("hivex-lockup-dark.svg", DARK, "#F5EFE3", "#9AA9C0"),
        ("hivex-lockup-light.svg", LIGHT, "#241A0E", "#8A6B3B"),
    ):
        body = "\n  ".join([
            f'<g transform="translate(2 22) scale(0.5)">{mark_body(p)}</g>',
            wordmark_group(290, 75, 1.5, word),
            tagline(291, 266, 24, tag, WORDMARK_W * 1.5 - 4),
        ])
        write(name, svg_doc(880, 300, mark_defs(p), body, title))

    # 5. vertical (stacked) lockups
    for name, p, word, tag in (
        ("hivex-lockup-vertical-dark.svg", DARK, "#F5EFE3", "#9AA9C0"),
        ("hivex-lockup-vertical-light.svg", LIGHT, "#241A0E", "#8A6B3B"),
    ):
        body = "\n  ".join([
            f'<g transform="translate(226.4 16.4) scale(0.6)">{mark_body(p)}</g>',
            wordmark_group(200, 352, 1.0, word),
            tagline(201, 496, 21, tag, WORDMARK_W - 4),
        ])
        write(name, svg_doc(760, 560, mark_defs(p), body, title))

    # 6. README
    write("README.md", README)


README = """# Hivex — Agentic Orchestration Harness · Logo Kit

One honeycomb cell = the **harness**. The bold **X** through its middle is the
orchestration spine, wired to a single glowing core; six satellite nodes are
the agents it coordinates. Amber-on-charcoal keeps the hive warmth with a
technical, instrument-panel feel.

## Files

| File | Use |
| --- | --- |
| `hivex-icon.svg` | Primary app icon / avatar (dark tile) |
| `hivex-glyph.svg` | Simplified mark for tiny sizes (favicon, 16–32 px) |
| `hivex-mark.svg` | Bare mark, bright amber — for dark backgrounds |
| `hivex-mark-light.svg` | Bare mark, deep amber — for light backgrounds |
| `hivex-mono.svg` | Single-ink mark for print / engraving |
| `hivex-lockup-dark.svg` / `-light.svg` | Horizontal wordmark lockups |
| `hivex-lockup-vertical-*.svg` | Stacked lockups |
| `generate.py` | Regenerates every SVG (`python3 generate.py`) |
| `exports/` | Ready-to-use PNG renders |

## Palette

| Role | Hex |
| --- | --- |
| Honey highlight | `#FFD44F` |
| Amber core | `#FF9330` |
| Ember (gradient end) | `#FF8A1E` |
| Deep amber (light bg) | `#B9750E` |
| Tile charcoal | `#0A0F18` → `#1A2434` |
| Warm ink (wordmark, dark) | `#F5EFE3` |
| Cool slate (tagline, dark) | `#9AA9C0` |

## Usage

- Clear space: keep at least 25 % of the mark's width free on all sides.
- Minimum sizes: icon 32 px, glyph 16 px, horizontal lockup 140 px wide.
- Don't recolor the mark outside the palette, rotate it, or place the bright
  amber mark on light backgrounds (use `hivex-mark-light.svg` instead).

The wordmark is drawn as vector strokes (no font dependency); only the small
tagline uses a system sans-serif, so it degrades gracefully.
"""

if __name__ == "__main__":
    build()
