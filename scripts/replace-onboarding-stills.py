#!/usr/bin/env python3
"""S3 stills: replace the retired lowercase wordmark (5 glyphs) in each
onboarding still's titlebar with the brand-correct one. Inpaints the word
box with per-row titlebar background, then draws 'hivex' centered on the
old glyph box in the sampled ink color."""
from collections import Counter
from PIL import Image, ImageDraw, ImageFont

DIR = "/Users/admin/Desktop/hivebot-main/web/public/media/onboarding"
# name -> (xL, xR, yTop, yBot) of the old wordmark glyphs (measured this session
# via Vision OCR boxes + pixel ink profiling); height drives the font size.
STILLS = {
    "provider-verify-still.png": (116, 178, 27, 44),
    "knowledge-base-still.png": (116, 178, 27, 44),
    "meet-office-still.png": (116, 178, 27, 44),
    "work-ships-still.png": (169, 216, 213, 229),
}
FONT = "/System/Library/Fonts/HelveticaNeue.ttc"
NEW = "hivex"

for name, (L, R, T, B) in STILLS.items():
    path = f"{DIR}/{name}"
    im = Image.open(path).convert("RGB")
    reg = im.crop((L - 1, T - 1, R + 1, B + 1))
    px = Counter(reg.getdata())
    bg = px.most_common(1)[0][0]
    ink = Counter({p: n for p, n in px.items()
                   if sum(abs(a - b) for a, b in zip(p, bg)) > 60}).most_common(1)[0][0]
    d = ImageDraw.Draw(im)
    for y in range(T - 2, B + 3):  # inpaint word box with per-row bg
        row = Counter(im.crop((L - 12, y, R + 12, y + 1)).getdata()).most_common(1)[0][0]
        d.rectangle((L - 2, y, R + 2, y + 1), fill=row)
    size = 30  # hunt size so 'hivex' ink height matches the old glyph height
    tgt = B - T + 1
    while size > 8:
        f = ImageFont.truetype(FONT, size)
        bb = d.textbbox((0, 0), NEW, font=f)
        if bb[3] - bb[1] <= tgt:
            break
        size -= 1
    w, h = bb[2] - bb[0], bb[3] - bb[1]
    d.text(((L + R) / 2 - w / 2 - bb[0], (T + B) / 2 - h / 2 - bb[1]), NEW, font=f, fill=ink)
    im.save(path)
    print(f"{name}: bg={bg} ink={ink} font={size}px drew '{NEW}' {w}x{h} at cx={(L+R)/2:.0f}")
