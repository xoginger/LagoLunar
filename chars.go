// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"math"
	"os"
	"strings"
)

func (m *model) charView(cv *canvas, t float64) {
	vw, vh := cv.w, cv.h
	prim, sec := colors[m.prim].C, colors[m.sec].C
	// el sprite más grande que quepa; panel al lado si hay espacio
	size, side := byte('t'), false
	for _, s := range []byte{'L', 'M', 'S'} {
		r, c := figRows(s), figCols(s)
		if r+4 <= vh && vw >= c+48 {
			size, side = s, true
			break
		}
		if r+13 <= vh && vw >= c+4 {
			size = s
			break
		}
	}
	rows := figRows(size)
	hor := min(vh-1, 3+rows)
	drawSceneAt(cv, t, hor)
	cv.text(-1, 1, "✦ CREA TU PERSONAJE ✦", RGB{255, 220, 120})
	scale := 1
	if size == 't' {
		scale = 2
	}
	fr := 0
	if int(t/4)%2 == 1 { // 4 s caminando, 4 s parado
		fr = 1 + int(t*3)%2
	}
	left := int(t/8)%2 == 1
	cx := vw / 2
	if side {
		cx = 2 + figCols(size)/2 + max(0, (vw-figCols(size)-48)/3)
	}
	drawFig(size, m.char, fr, left, prim, sec, func(dx, dy int, ch rune, c RGB) {
		for sy := 0; sy < scale; sy++ {
			for sx := 0; sx < scale; sx++ {
				x := cx + dx*scale + sx
				y := hor - 1 + dy*scale - (scale - 1 - sy)
				cv.set(x, y, ch, c)
				ry := 2*hor - 1 - y
				if ry >= hor && ry < vh {
					sh := int(math.Round(1.5 * math.Sin(float64(ry)*0.9+t*2)))
					if x+sh >= 0 && x+sh < vw {
						bg := cv.c[ry][x+sh].Bg
						cv.set(x+sh, ry, ch, lerp(bg, c, 0.35))
					}
				}
			}
		}
	})
	bw, bh := 44, 9
	x0, y0 := (vw-bw)/2, clamp(vh-bh-1, 0, vh)
	if side {
		x0, y0 = vw-bw-2, clamp((vh-bh)/2, 0, vh)
	}
	cv.box(x0, y0, bw, bh, RGB{6, 10, 30}, RGB{90, 110, 190})
	cv.text(x0+3, y0+1, fmt.Sprintf("%d/%d  %s", m.char+1, len(rosterNames), rosterNames[m.char]), RGB{235, 240, 255})
	opt := []struct {
		label, val string
		sw         RGB
	}{
		{"Personaje", rosterNames[m.char], RGB{-1, 0, 0}},
		{"Color", colors[m.prim].Name, prim},
		{"Acento", colors[m.sec].Name, sec},
	}
	for i, r := range opt {
		fg, pre := RGB{150, 160, 195}, "  "
		if i == m.row {
			fg, pre = RGB{255, 220, 120}, "› "
		}
		val := []rune(r.val)
		if len(val) > 19 {
			val = val[:19]
		}
		cv.text(x0+2, y0+3+i, fmt.Sprintf("%s%-9s ◂ %-19s ▸", pre, r.label, string(val)), fg)
		if r.sw.R >= 0 && i > 0 {
			cv.text(x0+bw-4, y0+3+i, "██", r.sw)
		}
	}
	cv.text(x0+3, y0+7, "←→ cambiar ↑↓ elegir Enter jugar", RGB{130, 140, 180})
	cv.text(1, vh-1, fmt.Sprintf(" ● %d en línea ", onlineCount()), RGB{120, 230, 140})
	cv.text(vw-12, vh-1, "Esc volver", RGB{110, 120, 160})
}

// gallery imprime todos los personajes en tamaño grande, para documentación.
func gallery() {
	size := byte('L')
	if len(os.Args) > 2 {
		size = os.Args[2][0]
	}
	cw, rh := figCols(size)+3, figRows(size)+3
	cv := newCanvas(cw*5+2, rh*2+2)
	cv.mode = cmTrue
	for y := range cv.c {
		for x := range cv.c[y] {
			cv.c[y][x] = TCell{' ', RGB{200, 200, 200}, RGB{6, 10, 30}}
		}
	}
	cv.text(-1, 0, "LAGO LUNAR · 10 personajes", RGB{255, 220, 120})
	for i := range rosterNames {
		bx, by := 1+(i%5)*cw, 1+(i/5)*rh
		drawFig(size, i, 0, false, RGB{-1, 0, 0}, RGB{-1, 0, 0}, func(dx, dy int, ch rune, c RGB) {
			cv.set(bx+cw/2+dx, by+figRows(size)+dy, ch, c)
		})
		cv.text(bx+(cw-len([]rune(rosterNames[i])))/2, by+figRows(size)+1, rosterNames[i], RGB{220, 225, 240})
	}
	os.Stdout.WriteString("\x1b[2J\x1b[H" + strings.ReplaceAll(cv.String(), "\n", "\r\n"))
}
