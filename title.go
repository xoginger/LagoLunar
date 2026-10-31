// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"math"
)

const GameName = "LAGO LUNAR"

var font = map[rune][]string{
	'L': {"██╗     ", "██║     ", "██║     ", "██║     ", "███████╗", "╚══════╝"},
	'A': {" █████╗ ", "██╔══██╗", "███████║", "██╔══██║", "██║  ██║", "╚═╝  ╚═╝"},
	'G': {" ██████╗ ", "██╔════╝ ", "██║  ███╗", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'O': {" ██████╗ ", "██╔═══██╗", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'U': {"██╗   ██╗", "██║   ██║", "██║   ██║", "██║   ██║", "╚██████╔╝", " ╚═════╝ "},
	'N': {"███╗   ██╗", "████╗  ██║", "██╔██╗ ██║", "██║╚██╗██║", "██║ ╚████║", "╚═╝  ╚═══╝"},
	'R': {"██████╗ ", "██╔══██╗", "██████╔╝", "██╔══██╗", "██║  ██║", "╚═╝  ╚═╝"},
	' ': {"   ", "   ", "   ", "   ", "   ", "   "},
}

func bigText(s string) []string {
	rows := make([]string, 6)
	for _, ch := range s {
		g := font[ch]
		for i := range rows {
			rows[i] += g[i]
		}
	}
	return rows
}

var bigTitle = bigText(GameName)

// TCell es una celda de pantalla con color de texto y de fondo.
type TCell struct {
	Ch     rune
	Fg, Bg RGB
}

// canvas es la pantalla que se dibuja en cada cuadro.
type canvas struct {
	w, h int
	c    [][]TCell
	mode colorMode
}

func newCanvas(w, h int) *canvas {
	cv := &canvas{w: w, h: h, c: make([][]TCell, h)}
	for y := range cv.c {
		cv.c[y] = make([]TCell, w)
	}
	return cv
}

func (cv *canvas) set(x, y int, ch rune, fg RGB) {
	if x >= 0 && y >= 0 && x < cv.w && y < cv.h {
		cv.c[y][x].Ch, cv.c[y][x].Fg = ch, fg
	}
}

// text escribe en x, o centrado si x<0, sin tocar el fondo.
func (cv *canvas) text(x, y int, s string, fg RGB) {
	r := []rune(s)
	if x < 0 {
		x = (cv.w - len(r)) / 2
	}
	for i, ch := range r {
		cv.set(x+i, y, ch, fg)
	}
}

func (cv *canvas) box(x0, y0, bw, bh int, bg RGB, border RGB) {
	for y := y0; y < y0+bh; y++ {
		for x := x0; x < x0+bw; x++ {
			if x < 0 || y < 0 || x >= cv.w || y >= cv.h {
				continue
			}
			ch := ' '
			switch {
			case (y == y0 || y == y0+bh-1) && (x == x0 || x == x0+bw-1):
				ch = map[bool]rune{true: '╭', false: '╰'}[y == y0]
				if x == x0+bw-1 {
					ch = map[bool]rune{true: '╮', false: '╯'}[y == y0]
				}
			case y == y0 || y == y0+bh-1:
				ch = '─'
			case x == x0 || x == x0+bw-1:
				ch = '│'
			}
			cv.c[y][x] = TCell{ch, border, bg}
		}
	}
}

func (cv *canvas) String() string { return cv.fastString() }

func hash(x, y int) float64 {
	n := uint32(x*374761393 + y*668265263)
	n = (n ^ (n >> 13)) * 1274126177
	return float64(n&0xffff) / 65535
}

func lerp(a, b RGB, k float64) RGB {
	return RGB{a.R + (b.R-a.R)*k, a.G + (b.G-a.G)*k, a.B + (b.B-a.B)*k}
}

// fondo del lago con luna; devuelve la fila del horizonte
func drawScene(cv *canvas, t float64) int {
	hor := cv.h * 11 / 20
	if hor < 3 {
		hor = cv.h / 2
	}
	return drawSceneAt(cv, t, hor)
}

func drawSceneAt(cv *canvas, t float64, hor int) int {
	w, h := cv.w, cv.h
	mx, my := w*4/5, 2 // centro de la luna
	moonCol := RGB{240, 236, 200}
	for y := 0; y < hor; y++ {
		k := float64(y) / float64(max(1, hor))
		bg := lerp(RGB{2, 3, 14}, RGB{14, 22, 58}, k)
		for x := 0; x < w; x++ {
			cell := TCell{' ', bg, bg}
			// halo de la luna
			dx, dy := float64(x-mx)/2, float64(y-my)
			d := math.Sqrt(dx*dx + dy*dy)
			if d < 9 {
				cell.Bg = lerp(bg, RGB{40, 48, 80}, (1-d/9)*0.6)
			}
			if hv := hash(x, y); hv > 0.985 && d > 4 {
				tw := 0.5 + 0.5*math.Sin(t*3+hv*900)
				cell.Ch = []rune{'.', '·', '*', '+'}[int(hv*1000)%4]
				cell.Fg = lerp(cell.Bg, RGB{220, 225, 255}, 0.3+0.7*tw)
			}
			cv.c[y][x] = cell
		}
	}
	moon := []string{" .-''-. ", "/  o   \\", "|    () |", "\\  .    /", " '-..-' "}
	moon[2] = "|   ()  |"
	for i, s := range moon {
		for j, ch := range s {
			if ch != ' ' {
				cv.set(mx-4+j, my-2+i, ch, moonCol)
			}
		}
	}
	// cerros en el horizonte
	for x := 0; x < w; x++ {
		hh := int(1.5 + 1.2*math.Sin(float64(x)*0.11) + 0.8*math.Sin(float64(x)*0.37+1))
		for i := 0; i < hh && hor-1-i >= 0; i++ {
			ch := []rune{'^', 'A', '▲', '^'}[int(hash(x, i)*4)]
			cv.c[hor-1-i][x] = TCell{ch, RGB{16, 60, 40}, RGB{5, 14, 18}}
		}
	}
	// agua: refleja el cielo con olas y el camino de luz de la luna
	for y := hor; y < h; y++ {
		depth := float64(y-hor) / float64(max(1, h-hor))
		shift := int(math.Round(1.8 * math.Sin(float64(y)*0.9+t*2.0)))
		wbg := lerp(RGB{8, 18, 48}, RGB{2, 6, 20}, depth)
		my2 := 2*hor - 1 - y
		for x := 0; x < w; x++ {
			cell := TCell{' ', wbg, wbg}
			sx := clamp(x+shift, 0, w-1)
			if my2 >= 0 && my2 < hor {
				s := cv.c[my2][sx]
				if s.Ch != ' ' && s.Ch != 0 {
					cell.Ch = s.Ch
					cell.Fg = lerp(wbg, s.Fg, 0.45)
				}
			}
			// camino de luz
			pw := 3 + depth*10
			if math.Abs(float64(x-mx)) < pw {
				sh := math.Sin(float64(x)*0.8+float64(y)*1.7-t*4) + math.Sin(float64(x)*0.23-t*1.3)
				if sh > 0.9 {
					cell.Ch = []rune{'~', '-', '=', '~'}[int(hash(x, y)*4)]
					cell.Fg = lerp(wbg, moonCol, 0.5+0.25*sh)
				}
			} else if wv := math.Sin(float64(x)*0.35 + float64(y)*2.3 - t*2.5); wv > 0.97 {
				cell.Ch = '~'
				cell.Fg = lerp(wbg, RGB{90, 130, 200}, 0.5)
			}
			cv.c[y][x] = cell
		}
	}
	return hor
}

func drawTitle(cv *canvas, t float64, hor int) int {
	tw := len([]rune(bigTitle[0]))
	y0 := max(5, hor/2-5)
	if cv.w >= tw+2 && hor >= y0+7 {
		x0 := (cv.w - tw) / 2
		for i, row := range bigTitle {
			for j, ch := range []rune(row) {
				if ch == ' ' {
					continue
				}
				k := 0.5 + 0.5*math.Sin(float64(j)*0.08-t*1.5)
				col := lerp(RGB{110, 160, 255}, RGB{235, 240, 255}, k)
				if ch != '█' {
					col = RGB{60, 80, 150}
				}
				cv.set(x0+j, y0+i, ch, col)
				// reflejo en el agua
				ry := 2*hor - 1 - (y0 + i)
				if ry < cv.h && ry >= hor {
					sh := int(math.Round(1.5 * math.Sin(float64(ry)*0.9+t*2)))
					if ch == '█' {
						bg := cv.c[ry][clamp(x0+j+sh, 0, cv.w-1)].Bg
						cv.set(x0+j+sh, ry, '▒', lerp(bg, col, 0.35))
					}
				}
			}
		}
		return y0 + 7
	}
	cv.text(-1, max(0, hor/2-1), "~ "+GameName+" ~", RGB{220, 230, 255})
	return hor/2 + 1
}
