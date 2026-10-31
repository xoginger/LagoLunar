# Lago Lunar - MMORPG ASCII por SSH
# Copyright (c) 2026 Xocotzin Granados
# Licencia MIT, ver LICENSE.

"""Dibuja los NPCs con figuras de PIL y los pasa a ASCII con el mismo tamaño que
los personajes (15x14 celdas). Escribe npcs_gen.go. Uso: python3 tools/gen_npcs.py"""
from PIL import Image, ImageDraw

COLS, ROWS = 15, 14
CW, CH = 8, 16                 # pixeles por celda
PW, PH = COLS * CW, ROWS * CH  # 120 x 224
CX = PW // 2
RAMP = " .:-=+*#%@"


def new():
    im = Image.new('RGBA', (PW, PH), (0, 0, 0, 0))
    return im, ImageDraw.Draw(im)


def shade(c, k):
    return tuple(max(0, min(255, int(v * k))) for v in c[:3]) + (255,)


def body(d, top, bot, wtop, wbot, c, shadow=True):
    d.polygon([(CX - wtop, top), (CX + wtop, top), (CX + wbot, bot), (CX - wbot, bot)], fill=c)
    if shadow:  # lado derecho en sombra
        d.polygon([(CX + wtop * 0.35, top), (CX + wtop, top), (CX + wbot, bot), (CX + wbot * 0.4, bot)], fill=shade(c, 0.7))


def head(d, cy, r, skin, hair=None, bald=False):
    d.ellipse([CX - r, cy - r, CX + r, cy + r], fill=skin)
    d.ellipse([CX + r * 0.2, cy - r, CX + r, cy + r], fill=shade(skin, 0.82))
    if hair and not bald:
        d.chord([CX - r - 1, cy - r - 2, CX + r + 1, cy + r * 0.6], 180, 360, fill=hair)


def legs(d, top, c, boots):
    d.rectangle([CX - 13, top, CX - 3, PH - 10], fill=c)
    d.rectangle([CX + 3, top, CX + 13, PH - 10], fill=shade(c, 0.75))
    d.rectangle([CX - 15, PH - 16, CX - 2, PH - 1], fill=boots)
    d.rectangle([CX + 2, PH - 16, CX + 15, PH - 1], fill=shade(boots, 0.8))


def arms(d, y, c, spread=30, down=70):
    d.polygon([(CX - 22, y), (CX - 16, y), (CX - spread + 8, y + down), (CX - spread, y + down)], fill=c)
    d.polygon([(CX + 16, y), (CX + 22, y), (CX + spread, y + down), (CX + spread - 8, y + down)], fill=shade(c, 0.75))


def hand(d, x, y, skin):
    d.ellipse([x - 5, y - 5, x + 5, y + 5], fill=skin)


SKIN = (225, 180, 140, 255)
SKIN2 = (190, 135, 95, 255)
SKIN3 = (150, 100, 70, 255)


def mercader(d):
    # mochila grande detrás de los hombros
    d.rounded_rectangle([CX - 40, 52, CX + 40, 150], 12, fill=(120, 80, 45, 255))
    d.rounded_rectangle([CX - 44, 40, CX + 44, 60], 8, fill=(170, 60, 50, 255))  # cobija enrollada
    d.line([CX - 44, 50, CX + 44, 50], fill=(110, 40, 35, 255), width=2)
    legs(d, 150, (90, 70, 50, 255), (60, 40, 25, 255))
    body(d, 58, 160, 24, 30, (150, 100, 60, 255))
    d.polygon([(CX - 14, 90), (CX + 14, 90), (CX + 20, 168), (CX - 20, 168)], fill=(230, 220, 190, 255))  # delantal
    d.rectangle([CX - 26, 120, CX + 26, 126], fill=(80, 50, 30, 255))
    d.ellipse([CX + 14, 122, CX + 30, 142], fill=(230, 190, 60, 255))  # bolsa de monedas
    arms(d, 62, (150, 100, 60, 255))
    hand(d, CX - 30, 134, SKIN)
    head(d, 36, 15, SKIN, (70, 45, 30, 255))
    d.ellipse([CX - 30, 18, CX + 30, 30], fill=(140, 100, 50, 255))  # sombrero
    d.rectangle([CX - 15, 4, CX + 15, 24], fill=(160, 115, 60, 255))
    d.rectangle([CX - 15, 18, CX + 15, 22], fill=(170, 60, 50, 255))
    d.ellipse([CX - 12, 46, CX + 12, 56], fill=(100, 65, 40, 255))  # barba
    return {'eyes': [(6, 2), (8, 2)]}


def mercader_real(d):
    d.polygon([(CX - 36, 60), (CX + 36, 60), (CX + 46, PH - 2), (CX - 46, PH - 2)], fill=(110, 30, 70, 255))  # capa
    body(d, 58, PH - 4, 24, 36, (150, 40, 90, 255))
    d.rectangle([CX - 3, 70, CX + 3, PH - 4], fill=(240, 200, 70, 255))  # franja dorada
    d.rectangle([CX - 36, PH - 12, CX + 36, PH - 4], fill=(240, 200, 70, 255))
    d.ellipse([CX - 30, 52, CX + 30, 74], fill=(235, 230, 220, 255))  # cuello de piel
    for x in range(-24, 25, 8):
        d.point((CX + x, 63), fill=(40, 40, 40, 255))
    arms(d, 66, (150, 40, 90, 255), 32, 64)
    hand(d, CX + 30, 132, SKIN)
    d.rectangle([CX + 26, 112, CX + 46, 116], fill=(240, 200, 70, 255))  # balanza
    d.line([CX + 36, 100, CX + 36, 128], fill=(240, 200, 70, 255), width=2)
    d.chord([CX + 22, 116, CX + 34, 126], 0, 180, fill=(240, 200, 70, 255))
    d.chord([CX + 38, 116, CX + 50, 126], 0, 180, fill=(240, 200, 70, 255))
    head(d, 36, 15, SKIN, (60, 40, 30, 255))
    d.rectangle([CX - 16, 8, CX + 16, 26], fill=(120, 30, 70, 255))  # gorro
    d.rectangle([CX - 18, 22, CX + 18, 27], fill=(240, 200, 70, 255))
    d.polygon([(CX + 8, 10), (CX + 30, -6), (CX + 14, 14)], fill=(250, 250, 250, 255))  # pluma
    return {'eyes': [(6, 2), (8, 2)]}


def abuela(d):
    # bastón con luna
    d.line([CX + 34, 40, CX + 34, PH - 2], fill=(130, 90, 50, 255), width=5)
    d.ellipse([CX + 24, 14, CX + 46, 36], fill=(245, 235, 170, 255))
    d.ellipse([CX + 30, 12, CX + 50, 32], fill=(0, 0, 0, 0))
    body(d, 70, PH - 2, 22, 34, (60, 70, 130, 255))  # falda larga
    d.polygon([(CX - 30, 62), (CX + 30, 62), (CX + 22, 120), (CX - 22, 120)], fill=(110, 60, 120, 255))  # rebozo
    for x in range(-22, 23, 6):
        d.line([CX + x, 118, CX + x, 126], fill=(200, 160, 220, 255))
    arms(d, 70, (110, 60, 120, 255), 34, 50)
    hand(d, CX + 33, 122, SKIN2)
    hand(d, CX - 33, 122, SKIN2)
    head(d, 44, 14, SKIN2, (200, 200, 205, 255))
    d.ellipse([CX - 10, 18, CX + 10, 34], fill=(215, 215, 220, 255))  # chongo
    d.line([CX - 9, 54, CX + 9, 54], fill=(150, 100, 80, 255))
    return {'eyes': [(6, 2), (8, 2)], 'eyerow': 2, 'glow': [(12, 1), (13, 1)]}


def capitan(d):
    d.polygon([(CX - 30, 58), (CX + 30, 58), (CX + 40, PH - 18), (CX - 40, PH - 18)], fill=(170, 30, 40, 255))  # capa
    legs(d, 150, (150, 155, 165, 255), (90, 90, 100, 255))
    body(d, 56, 156, 26, 22, (185, 190, 200, 255))  # coraza
    d.line([CX - 18, 80, CX + 18, 80], fill=(120, 125, 135, 255), width=2)
    d.line([CX - 18, 110, CX + 18, 110], fill=(120, 125, 135, 255), width=2)
    d.rectangle([CX - 24, 138, CX + 24, 146], fill=(110, 70, 40, 255))  # cinturón
    d.ellipse([CX - 34, 52, CX - 14, 70], fill=(200, 205, 215, 255))  # hombreras
    d.ellipse([CX + 14, 52, CX + 34, 70], fill=(160, 165, 175, 255))
    arms(d, 62, (175, 180, 190, 255), 32, 64)
    d.line([CX + 30, 128, CX + 34, PH - 30], fill=(220, 225, 235, 255), width=4)  # espada
    d.rectangle([CX + 22, 124, CX + 40, 128], fill=(220, 180, 60, 255))
    d.rounded_rectangle([CX - 52, 90, CX - 22, 150], 10, fill=(60, 80, 150, 255))  # escudo
    d.line([CX - 37, 96, CX - 37, 144], fill=(230, 200, 80, 255), width=3)
    d.line([CX - 48, 112, CX - 26, 112], fill=(230, 200, 80, 255), width=3)
    head(d, 36, 15, SKIN, None)
    d.chord([CX - 17, 14, CX + 17, 50], 180, 360, fill=(170, 175, 185, 255))  # casco
    d.rectangle([CX - 17, 30, CX + 17, 34], fill=(140, 145, 155, 255))
    d.polygon([(CX - 3, 16), (CX + 3, 16), (CX + 14, -2), (CX + 4, -2)], fill=(200, 40, 50, 255))  # penacho
    d.ellipse([CX - 12, 44, CX + 12, 54], fill=(110, 70, 40, 255))  # bigote
    return {'eyes': [(6, 2), (8, 2)]}


def sabio(d):
    body(d, 56, PH - 2, 22, 36, (40, 130, 130, 255))  # túnica
    for y in range(90, PH - 8, 22):
        d.line([CX - 28 + (y - 90) // 10, y, CX + 28 - (y - 90) // 10, y], fill=(230, 200, 90, 255), width=3)
        for x in range(-24, 25, 12):
            d.polygon([(CX + x, y + 4), (CX + x + 4, y + 10), (CX + x - 4, y + 10)], fill=(200, 90, 60, 255))
    arms(d, 62, (40, 130, 130, 255), 26, 56)
    d.rectangle([CX - 22, 108, CX + 2, 130], fill=(120, 60, 40, 255))  # libro
    d.rectangle([CX - 20, 110, CX, 128], fill=(240, 230, 200, 255))
    d.line([CX - 10, 110, CX - 10, 128], fill=(120, 60, 40, 255))
    head(d, 36, 15, SKIN3, None)
    d.polygon([(CX - 14, 46), (CX + 14, 46), (CX + 6, 96), (CX - 6, 96)], fill=(240, 240, 240, 255))  # barba
    d.polygon([(CX - 20, 26), (CX + 20, 26), (CX + 6, -10), (CX - 6, -10)], fill=(30, 100, 110, 255))  # sombrero alto
    d.rectangle([CX - 20, 22, CX + 20, 28], fill=(230, 200, 90, 255))
    return {'eyes': [(6, 2), (8, 2)]}


def monje(d):
    body(d, 56, PH - 2, 24, 34, (220, 110, 40, 255))
    d.polygon([(CX - 24, 56), (CX + 6, 56), (CX + 30, 140), (CX + 20, 146)], fill=(170, 70, 30, 255))  # banda
    d.rectangle([CX - 26, 128, CX + 26, 133], fill=(230, 210, 150, 255))  # cordón
    d.line([CX - 10, 133, CX - 14, 168], fill=(230, 210, 150, 255), width=3)
    arms(d, 62, (220, 110, 40, 255), 14, 52)
    d.ellipse([CX - 12, 104, CX + 12, 124], fill=SKIN2)  # manos juntas
    for i in range(7):  # rosario
        d.ellipse([CX - 18 + i * 6, 70 + abs(i - 3) * 6, CX - 14 + i * 6, 74 + abs(i - 3) * 6], fill=(90, 50, 30, 255))
    d.ellipse([CX - 8, 98, CX + 8, 112], fill=(255, 150, 40, 255))  # brasa
    d.ellipse([CX - 4, 100, CX + 4, 108], fill=(255, 230, 120, 255))
    head(d, 36, 15, SKIN2, None, bald=True)
    d.arc([CX - 12, 22, CX + 12, 40], 200, 340, fill=(250, 220, 190, 255), width=2)
    return {'eyes': [(6, 2), (8, 2)], 'glow': [(7, 6)]}


def marisol(d):
    legs(d, 170, SKIN, (120, 70, 40, 255))
    body(d, 58, 176, 20, 38, (230, 190, 60, 255))  # vestido
    d.polygon([(CX - 16, 96), (CX + 16, 96), (CX + 24, 172), (CX - 24, 172)], fill=(245, 245, 235, 255))  # delantal
    for x in range(-30, 31, 10):
        d.ellipse([CX + x - 3, 160, CX + x + 3, 166], fill=(200, 80, 60, 255))
    arms(d, 64, SKIN, 30, 52)
    d.chord([CX + 18, 104, CX + 50, 136], 0, 180, fill=(170, 120, 60, 255))  # canasta
    d.arc([CX + 20, 96, CX + 48, 124], 180, 360, fill=(140, 95, 45, 255), width=3)
    for x, c in ((24, (230, 60, 60)), (32, (250, 170, 40)), (40, (120, 200, 80))):
        d.ellipse([CX + x - 4, 112, CX + x + 4, 120], fill=c + (255,))
    head(d, 36, 15, SKIN, (60, 30, 20, 255))
    d.chord([CX - 18, 16, CX + 18, 46], 180, 360, fill=(60, 150, 120, 255))  # pañuelo
    d.polygon([(CX - 18, 30), (CX - 26, 52), (CX - 14, 46)], fill=(60, 150, 120, 255))
    d.polygon([(CX - 20, 36), (CX - 14, 36), (CX - 10, 70), (CX - 20, 66)], fill=(60, 30, 20, 255))  # trenza
    return {'eyes': [(6, 2), (8, 2)]}


def caravana(d):
    legs(d, 148, (110, 90, 70, 255), (80, 50, 30, 255))
    d.polygon([(CX - 24, 58), (CX + 24, 58), (CX + 34, 190), (CX - 34, 190)], fill=(150, 110, 70, 255))  # abrigo
    d.polygon([(CX + 8, 58), (CX + 24, 58), (CX + 34, 190), (CX + 14, 190)], fill=(115, 85, 55, 255))
    d.line([CX - 24, 64, CX + 26, 138], fill=(80, 50, 30, 255), width=4)  # correa
    d.rounded_rectangle([CX + 16, 128, CX + 40, 152], 4, fill=(110, 70, 40, 255))  # morral
    d.rectangle([CX - 24, 132, CX + 18, 138], fill=(80, 50, 30, 255))
    arms(d, 62, (150, 110, 70, 255), 30, 60)
    d.rectangle([CX - 40, 112, CX - 24, 132], fill=(235, 220, 170, 255))  # mapa
    d.line([CX - 38, 118, CX - 26, 118], fill=(150, 60, 40, 255))
    head(d, 38, 15, SKIN2, (30, 20, 15, 255))
    d.polygon([(CX - 16, 44), (CX + 16, 44), (CX + 18, 60), (CX - 18, 60)], fill=(200, 70, 50, 255))  # pañoleta
    d.ellipse([CX - 34, 20, CX + 34, 32], fill=(120, 85, 50, 255))  # sombrero
    d.chord([CX - 16, 2, CX + 16, 34], 180, 360, fill=(140, 100, 60, 255))
    d.rectangle([CX - 16, 22, CX + 16, 25], fill=(60, 40, 25, 255))
    return {'eyes': [(6, 2), (8, 2)], 'eyerow': 2}


NPCS = [("Mercader", mercader), ("Mercader Real", mercader_real), ("Abuela Luna", abuela),
        ("Capitán Roldán", capitan), ("Sabio Ixtli", sabio), ("Hermano Ascua", monje),
        ("Marisol", marisol), ("Jefa de Caravana", caravana)]


def to_ascii(im, seed):
    import random
    rnd = random.Random(seed)
    px = im.load()
    G, C = [], []
    for r in range(ROWS):
        g, c = '', ''
        for k in range(COLS):
            pts = []
            for y in range(CH):
                for x in range(CW):
                    p = px[k * CW + x, r * CH + y]
                    if p[3] > 0:
                        pts.append((x, y, p))
            cov = len(pts) / (CW * CH)
            if cov < 0.18:
                g += ' '; c += '000000'
                continue
            n = len(pts)
            col = tuple(sum(p[2][i] for p in pts) / n for i in range(3))
            L = lambda p: (0.3 * p[0] + 0.59 * p[1] + 0.11 * p[2]) / 255
            lum = L(col)

            def mean(sel, f):
                v = [f(p) for p in pts if sel(p)]
                return sum(v) / len(v) if v else None
            if cov < 0.62:
                # borde: el carácter sigue la forma de la silueta
                cnt = lambda sel: sum(1 for p in pts if sel(p))
                left, right = cnt(lambda p: p[0] < CW // 2), cnt(lambda p: p[0] >= CW // 2)
                top, bot = cnt(lambda p: p[1] < CH // 2), cnt(lambda p: p[1] >= CH // 2)
                if top < bot * 0.45:
                    ch = '_' if cov < 0.35 else '.'
                elif bot < top * 0.45:
                    ch = '`' if cov < 0.35 else "'"
                elif left > right * 1.8:
                    ch = '|' if min(top, bot) > max(top, bot) * 0.7 else ('\\' if top > bot else '/')
                elif right > left * 1.8:
                    ch = '|' if min(top, bot) > max(top, bot) * 0.7 else ('/' if top > bot else '\\')
                else:
                    ch = ':'
            else:
                lt = mean(lambda p: p[1] < CH // 2, lambda p: L(p[2]))
                lb = mean(lambda p: p[1] >= CH // 2, lambda p: L(p[2]))
                ll = mean(lambda p: p[0] < CW // 2, lambda p: L(p[2]))
                lr = mean(lambda p: p[0] >= CW // 2, lambda p: L(p[2]))
                dv = (lt or lum) - (lb or lum)
                dh = (ll or lum) - (lr or lum)
                if max(abs(dv), abs(dh)) > 0.12:
                    # borde interno entre dos colores
                    if abs(dv) > 2 * abs(dh):
                        ch = '-' if dv > 0 else '_'
                    elif abs(dh) > 2 * abs(dv):
                        ch = '|'
                    else:
                        ch = '/' if dv * dh < 0 else '\\'
                else:
                    sets = [':;', '-=+', '=+*', '*#%', '#%@']
                    st = sets[min(4, int(lum * 5))]
                    ch = rnd.choice(st)
            k2 = 1.12 if lum < 0.35 else 1.0  # aclarar un poco los oscuros
            g += ch
            c += '%02x%02x%02x' % tuple(min(255, int(v * k2)) for v in col)
        G.append(g); C.append(c)
    return G, C


def put(G, C, col, row, ch, rgb):
    G[row] = G[row][:col] + ch + G[row][col + 1:]
    C[row] = C[row][:col * 6] + '%02x%02x%02x' % rgb + C[row][col * 6 + 6:]


out = ['// Lago Lunar - MMORPG ASCII por SSH', '// Copyright (c) 2026 Xocotzin Granados', '// Licencia MIT, ver LICENSE.', '',
       '// Code generated by tools/gen_npcs.py; DO NOT EDIT.', '', 'package main', '',
       'var npcSprites = map[string][2]BigFrame{']
for name, fn in NPCS:
    im, d = new()
    meta = fn(d)
    G, C = to_ascii(im, name)
    frames = []
    for blink in (False, True):
        g, c = list(G), list(C)
        er = meta.get('eyerow', 2)
        for (x, _) in meta['eyes']:
            put(g, c, x, er, '-' if blink else 'o', (200, 150, 120) if blink else (245, 245, 235))
        for (x, y) in meta.get('glow', []):
            put(g, c, x, y, '*' if not blink else '+', (255, 240, 150) if not blink else (255, 200, 90))
        frames.append((g, c))
    out.append('\t%s: {' % ('"' + name + '"'))
    for g, c in frames:
        esc = lambda s: s.replace('\\', '\\\\').replace('"', '\\"')
        out.append('\t\t{G: []string{%s}, C: []string{%s}},' % (', '.join('"%s"' % esc(x) for x in g), ', '.join('"%s"' % x for x in c)))
    out.append('\t},')
out.append('}')
open('npcs_gen.go', 'w').write('\n'.join(out) + '\n')
