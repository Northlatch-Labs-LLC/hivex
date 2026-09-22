#!/usr/bin/env python3
"""S3 GIFs: replace the retired lowercase wordmark in each onboarding GIF's
title bar with the brand-correct one, on every composited frame. Same
inpaint+redraw as replace-onboarding-stills.py, per-frame bg/ink sampling."""
import sys
from collections import Counter
from PIL import Image, ImageDraw, ImageFont

DIR = "/Users/admin/Desktop/hivebot-main/web/public/media/onboarding"
BOXES = {  # measured this session: Vision OCR boxes + ink profiling; GIF dims == still dims
    "provider-verify.gif": (116, 178, 27, 44),
    "knowledge-base.gif": (116, 178, 27, 44),
    "meet-office.gif": (116, 178, 27, 44),
    "work-ships.gif": (169, 216, 213, 229),
}
FONT = "/System/Library/Fonts/HelveticaNeue.ttc"
NEW = "hivex"

for name, (L, R, T, B) in BOXES.items():
    if sys.argv[1:] and name not in sys.argv[1:]:
        continue
    path = f"{DIR}/{name}"
    src = Image.open(path)
    loop = src.info.get("loop", 0)
    frames, durs = [], []
    for i in range(src.n_frames):
        src.seek(i)
        durs.append(src.info.get("duration", 100))
        im = src.convert("RGB")  # composited full frame
        reg = im.crop((L - 1, T - 1, R + 1, B + 1))
        px = Counter(reg.getdata())
        bg = px.most_common(1)[0][0]
        inkc = Counter({p: n for p, n in px.items()
                        if sum(abs(a - b) for a, b in zip(p, bg)) > 60})
        if not inkc:  # frame without the wordmark (box already bg) — keep as-is
            frames.append(im); continue
        ink = inkc.most_common(1)[0][0]
        d = ImageDraw.Draw(im)
        for y in range(T - 2, B + 3):
            row = Counter(im.crop((L - 12, y, R + 12, y + 1)).getdata()).most_common(1)[0][0]
            d.rectangle((L - 2, y, R + 2, y + 1), fill=row)
        size, f, bb = 30, None, None
        while size > 8:
            f = ImageFont.truetype(FONT, size)
            bb = d.textbbox((0, 0), NEW, font=f)
            if bb[3] - bb[1] <= B - T + 1: break
            size -= 1
        d.text(((L + R) / 2 - (bb[2] - bb[0]) / 2 - bb[0],
                (T + B) / 2 - (bb[3] - bb[1]) / 2 - bb[1]), NEW, font=f, fill=ink)
        frames.append(im)
    # one shared palette across frames -> Pillow can emit delta frames (small file)
    base = frames[0].quantize(colors=256, method=Image.MEDIANCUT)
    frames = [base] + [fr.quantize(palette=base, colors=256,
                                    dither=Image.Dither.NONE) for fr in frames[1:]]
    for fr in frames:  # composited frames are opaque; bad transparency info breaks save
        fr.info.pop("transparency", None)
    frames[0].save(path, save_all=True, append_images=frames[1:],
                   duration=durs, loop=loop, optimize=False)
    print(f"{name}: {len(frames)} frames patched, durations {min(durs)}-{max(durs)}ms, loop={loop}")
