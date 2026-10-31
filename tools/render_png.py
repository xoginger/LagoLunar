# Lago Lunar - MMORPG ASCII por SSH
# Copyright (c) 2026 Xocotzin Granados
# Licencia MIT, ver LICENSE.

"""Pasa una captura ANSI a PNG con pyte: render_png.py entrada.ansi salida.png [cols] [filas]"""
import pyte, sys
from PIL import Image, ImageDraw, ImageFont
COLS=int(sys.argv[3]) if len(sys.argv)>3 else 100; ROWS=int(sys.argv[4]) if len(sys.argv)>4 else 30
scr = pyte.Screen(COLS, ROWS); st = pyte.ByteStream(scr); st.feed(open(sys.argv[1],'rb').read())
import glob
fp = (glob.glob('/usr/share/fonts/**/DejaVuSansMono.ttf', recursive=True) or [None])[0]
font = ImageFont.truetype(fp, 14) if fp else ImageFont.load_default()
cw, ch = 9, 17
img = Image.new('RGB', (COLS*cw, ROWS*ch), (5,8,20)); d = ImageDraw.Draw(img)
def col(c, dflt):
    if c == 'default': return dflt
    try: return tuple(int(c[i:i+2],16) for i in (0,2,4))
    except Exception: return dflt
for y in range(ROWS):
    for x in range(COLS):
        c = scr.buffer[y][x]
        fg, bg = col(c.fg,(220,220,220)), col(c.bg,(5,8,20))
        if c.reverse: fg, bg = bg, fg
        d.rectangle([x*cw,y*ch,(x+1)*cw,(y+1)*ch], fill=bg); d.text((x*cw,y*ch), c.data, font=font, fill=fg)
img.save(sys.argv[2]); print("\n".join(scr.display))
