# Hivex — Agentic Orchestration Harness · Logo Kit

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
