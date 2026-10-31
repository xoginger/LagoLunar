# Lago Lunar - MMORPG ASCII por SSH
# Copyright (c) 2026 Xocotzin Granados
# Licencia MIT, ver LICENSE.

"""Convierte los 5 personajes de una imagen de referencia (ref2.png, no incluida)
más 5 variantes recoloreadas en sprites ASCII de varios tamaños, con 3 cuadros
(parado, paso A, paso B). Escribe sprites_gen.go. Necesita numpy, scipy y Pillow."""
import colorsys, math, json, sys
import numpy as np
from PIL import Image, ImageDraw
from scipy import ndimage as ndi

src = Image.open('ref2.png').convert('RGB'); A = np.array(src).astype(float)
CROPS = [(100, 326), (308, 518), (502, 730), (712, 930), (898, 1106)]
Y0, Y1 = 48, 596
PAD = 60

def segment(x0, x1):
    a = A[Y0:Y1, x0:x1]
    h, w, _ = a.shape
    L = np.median(a[:, :10], 1); R = np.median(a[:, -10:], 1)
    t = np.linspace(0, 1, w)[None, :, None]
    bg = L[:, None, :] * (1 - t) + R[:, None, :] * t
    d = np.sqrt(((a - bg) ** 2).sum(2))
    mx = a.max(2); mn = a.min(2); sat = (mx - mn) / (mx + 1)
    fg = (d > 26) | ((sat > 0.35) & (mx > 60))
    fg = ndi.binary_closing(fg, iterations=2); fg = ndi.binary_opening(fg, iterations=1)
    lab, n = ndi.label(fg)
    if n:
        sizes = ndi.sum(fg, lab, range(1, n + 1))
        # quedarse con la mancha más grande (y las grandes que tocan el centro)
        keep = np.zeros(n + 1, bool); keep[1 + int(np.argmax(sizes))] = True
        fg = keep[lab]
    fg = ndi.binary_fill_holes(fg)
    return a, fg

def hue_rotate(a, fg, deg):
    out = a.copy()
    h, w, _ = a.shape
    flat = a.reshape(-1, 3) / 255; m = fg.reshape(-1)
    for i in np.where(m)[0]:
        r, g, b = flat[i]; hh, ll, ss = colorsys.rgb_to_hls(r, g, b)
        if ss > 0.25:
            r, g, b = colorsys.hls_to_rgb((hh + deg / 360) % 1, ll, ss)
            out.reshape(-1, 3)[i] = (r * 255, g * 255, b * 255)
    return out

def pad(a, fg):
    h, w, _ = a.shape
    A2 = np.zeros((h, w + 2 * PAD, 3)); F2 = np.zeros((h, w + 2 * PAD), bool)
    A2[:, PAD:PAD + w] = a; F2[:, PAD:PAD + w] = fg
    return A2, F2

def hands(fg):
    h, w = fg.shape
    band = fg[int(h * .40):int(h * .58)]
    xs = np.where(band.any(0))[0]
    lx, rx = xs.min(), xs.max()
    ly = int(h * .40) + np.where(band[:, lx])[0].mean(); ry = int(h * .40) + np.where(band[:, rx])[0].mean()
    return (lx + 4, int(ly)), (rx - 4, int(ry))

def draw_weapon(a, fg, kind):
    h, w = fg.shape
    img = Image.fromarray(a.clip(0, 255).astype('uint8')); m = Image.fromarray((fg * 255).astype('uint8'))
    di, dm = ImageDraw.Draw(img), ImageDraw.Draw(m)
    (lx, ly), (rx, ry) = hands(fg)
    def L(p, q, wd, col):
        di.line([p, q], fill=col, width=wd); dm.line([p, q], fill=255, width=wd)
    def E(c, r, col):
        b = [c[0] - r, c[1] - r, c[0] + r, c[1] + r]; di.ellipse(b, fill=col); dm.ellipse(b, fill=255)
    if kind == 'sword':
        L((rx, ry + 20), (rx + 20, ry - 230), 9, (215, 220, 230)); L((rx - 22, ry - 4), (rx + 24, ry + 2), 9, (200, 160, 60))
    elif kind == 'bow':
        di.arc((lx - 70, ly - 190, lx + 20, ly + 190), 100, 260, fill=(150, 110, 60), width=9)
        dm.arc((lx - 70, ly - 190, lx + 20, ly + 190), 100, 260, fill=255, width=9)
        L((lx - 4, ly - 186), (lx - 4, ly + 186), 2, (230, 230, 230))
    elif kind == 'axe':
        L((rx, ry + 60), (rx + 6, ry - 180), 9, (120, 85, 50))
        di.polygon([(rx + 6, ry - 170), (rx + 56, ry - 200), (rx + 60, ry - 120), (rx + 6, ry - 140)], fill=(190, 195, 205))
        dm.polygon([(rx + 6, ry - 170), (rx + 56, ry - 200), (rx + 60, ry - 120), (rx + 6, ry - 140)], fill=255)
    elif kind == 'staff':
        L((rx, ry + 90), (rx + 4, ry - 220), 8, (170, 140, 110)); E((rx + 4, ry - 232), 18, (150, 230, 255))
    elif kind == 'scythe':
        L((rx, ry + 90), (rx + 10, ry - 220), 8, (60, 50, 60))
        di.arc((rx - 120, ry - 270, rx + 20, ry - 150), 200, 340, fill=(200, 200, 215), width=12)
        dm.arc((rx - 120, ry - 270, rx + 20, ry - 150), 200, 340, fill=255, width=12)
    return np.array(img).astype(float), np.array(m) > 127

def walk(a, fg, phase):
    """fase 0 parado; +1/-1 caminando: un pie se levanta y el cuerpo baja un poco."""
    if phase == 0:
        return a, fg
    h, w = fg.shape
    hip = int(h * .56); cx = w // 2
    A2 = np.zeros_like(a); F2 = np.zeros_like(fg)
    bob = 4
    A2[bob:hip + bob] = a[:hip]; F2[bob:hip + bob] = fg[:hip]   # el torso baja
    for side in (0, 1):
        xs = slice(0, cx) if side == 0 else slice(cx, w)
        lift = (side == 0) == (phase > 0)
        seg_a = a[hip:, xs]; seg_f = fg[hip:, xs]
        nh = seg_a.shape[0] - (24 if lift else 0)
        ia = np.array(Image.fromarray(seg_a.clip(0, 255).astype('uint8')).resize((seg_a.shape[1], nh)))
        im = np.array(Image.fromarray((seg_f * 255).astype('uint8')).resize((seg_f.shape[1], nh))) > 127
        shift = (-8 if side == 0 else 8) * (1 if lift else -0.4)
        ia = np.roll(ia, int(shift), 1); im = np.roll(im, int(shift), 1)
        top = hip + bob
        hh = min(nh, h - top)
        ra = A2[top:top + hh, xs]; rf = F2[top:top + hh, xs]
        ra[im[:hh]] = ia[:hh][im[:hh]]; rf |= im[:hh]
    return A2, F2

RAMP = " .,:;-=+*#%@"
def to_ascii(a, fg, rows, cols):
    h, w = fg.shape
    lum = (0.3 * a[..., 0] + 0.59 * a[..., 1] + 0.11 * a[..., 2]) / 255
    if fg.any():  # ecualizar dentro de la figura para usar toda la rampa
        v = np.sort(lum[fg]); lum = np.searchsorted(v, lum) / len(v)
    gy, gx = np.gradient(lum)
    G, C = [], []
    for r in range(rows):
        g, c = '', ''
        for k in range(cols):
            y0, y1 = int(r * h / rows), int((r + 1) * h / rows); x0, x1 = int(k * w / cols), int((k + 1) * w / cols)
            cm = fg[y0:y1, x0:x1]; cov = cm.mean()
            if cov < 0.22:
                g += ' '; c += '000000'; continue
            col = a[y0:y1, x0:x1][cm].mean(0)
            # aclarar para que se lea en terminal
            l = 0.3 * col[0] + 0.59 * col[1] + 0.11 * col[2]
            boost = min(3.2, max(1.0, 195 / (l + 1)))
            col = np.clip(col * boost, 0, 255)
            L = lum[y0:y1, x0:x1][cm].mean()
            hh_, ll_, ss_ = colorsys.rgb_to_hls(*(col / 255))
            col = np.array(colorsys.hls_to_rgb(hh_, 0.28 + 0.52 * L, min(1, ss_ * 1.35))) * 255
            if cov < 0.6:
                hy = cm[cm.shape[0] // 2:].mean() - cm[:cm.shape[0] // 2].mean()
                hx = cm[:, cm.shape[1] // 2:].mean() - cm[:, :cm.shape[1] // 2].mean()
                if abs(hx) > 2 * abs(hy): ch = '|' if cov > .35 else ':'
                elif abs(hy) > 2 * abs(hx): ch = '_' if hy > 0 else '-'
                else: ch = '/' if hx * hy < 0 else '\\'
            else:
                ex, ey = gx[y0:y1, x0:x1][cm].mean() * w / cols, gy[y0:y1, x0:x1][cm].mean() * h / rows
                if math.hypot(ex, ey) > 0.5:
                    ang = math.degrees(math.atan2(ey, ex)) % 180
                    ch = '|' if ang < 22 or ang > 158 else '\\' if ang < 68 else '=' if ang < 112 else '/'
                else:
                    ch = RAMP[min(len(RAMP) - 1, max(3, int(L * (len(RAMP) - 0.01))))]
            g += ch; c += '%02x%02x%02x' % tuple(int(v) for v in col)
        G.append(g); C.append(c)
    return G, C

ROSTER = [
    ('Guerrero Plateado', 0, 0, None), ('Paladina Élfica', 1, 0, None), ('Caballero de Bronce', 2, 0, None),
    ('Hechicera Violeta', 3, 0, None), ('Súcubo Carmesí', 4, 0, None),
    ('Espadachín', 0, 140, 'sword'), ('Arquera Élfica', 1, 95, 'bow'), ('Mercenario', 2, -25, 'axe'),
    ('Maga de Hielo', 3, -85, 'staff'), ('Reina Demonio', 4, -95, 'scythe'),
]
SIZES = {'S': 14, 'M': 22, 'L': 34}
base = []
for name, fi, rot, weap in ROSTER:
    c0, c1 = CROPS[fi]; mid = (c0 + c1) // 2
    a, fg = segment(mid - 112, mid + 112)
    if rot: a = hue_rotate(a, fg, rot)
    a, fg = pad(a, fg)
    if weap: a, fg = draw_weapon(a, fg, weap)
    base.append((a, fg))
# caja común para que todos los sprites tengan el mismo ancho
xs = np.where(np.any([f.any(0) for _, f in base], 0))[0]
X0, X1 = xs.min(), xs.max() + 1
out = ['// Lago Lunar - MMORPG ASCII por SSH', '// Copyright (c) 2026 Xocotzin Granados', '// Licencia MIT, ver LICENSE.', '',
       '// Code generated by tools/gen_ref.py from ref2.png; DO NOT EDIT.', '', 'package main', '',
       'var rosterNames = []string{' + ', '.join('"%s"' % r[0] for r in ROSTER) + '}', '',
       'var bigSprites = map[byte][][3]BigFrame{']
for key, rows in SIZES.items():
    h, w = base[0][1].shape
    cols = int(round(rows * 2.0 * (X1 - X0) / h))
    cols += cols % 2 == 0
    out.append("\t'%s': {" % key)
    for i, (a, fg) in enumerate(base):
        out.append('\t\t{')
        for j, ph in enumerate((0, 1, -1)):
            a2, f2 = walk(a, fg, ph)
            G, C = to_ascii(a2[:, X0:X1], f2[:, X0:X1], rows, cols)
            esc = lambda s: s.replace('\\', '\\\\').replace('"', '\\"')
            out.append('\t\t\t{G: []string{%s}, C: []string{%s}},' % (', '.join('"%s"' % esc(x) for x in G), ', '.join('"%s"' % x for x in C)))
        out.append('\t\t},')
    out.append('\t},')
out.append('}')
open('sprites_gen.go', 'w').write('\n'.join(out) + '\n')
print('cols', {k: int(round(r * 2.0 * (X1 - X0) / base[0][1].shape[0])) for k, r in SIZES.items()})
