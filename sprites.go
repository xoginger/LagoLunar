// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"math"
	"strings"
)

// Un sprite chico es de 3x3 y mira a la derecha. La fila 2 (piernas) tiene dos cuadros de caminar.
// Máscara: P primario, S secundario, K piel, N metal, E ojos/brillo, W hueso, ' ' transparente.
type Sprite struct {
	Name  string
	Rows  [2][3]string // [cuadro][fila]
	Masks [2][3]string
}

func mk(name, r0, m0, r1, m1, la, lb, ml string) Sprite {
	return Sprite{name, [2][3]string{{r0, r1, la}, {r0, r1, lb}}, [2][3]string{{m0, m1, ml}, {m0, m1, ml}}}
}

var sprites = []Sprite{
	mk("Caballero", "[=]", "NEN", "{#/", "SPN", "/ \\", "| |", "PPP"),
	mk("Mago", "/*\\", "PSP", "<#!", "KPN", "/_\\", "/ \\", "PPP"),
	mk("Arquera", "'o'", "SKS", "-|}", "NPS", "/ \\", "| |", "PPP"),
	mk("Ninja", "(-)", "PEP", "/#*", "PPN", "/ \\", "< >", "PPP"),
	mk("Pirata", "<o>", "SKS", "/#?", "KPN", "/ !", "| !", "PPN"),
	mk("Robot", "[0]", "NEN", "=H=", "SPS", "| |", "/ \\", "NNN"),
	mk("Esqueleto", " O ", " W ", "-E-", "WPW", "/ \\", "| |", "WWW"),
	mk("Hada", " o ", " K ", "<%>", "SPS", " ^ ", " v ", " P "),
	mk("Samurái", "^o^", "SKS", "[#-", "SPN", "/^\\", "/ \\", "PPP"),
	mk("Monje", "(o)", "SKS", "/O\\", "PSP", "|_|", "/_\\", "PPP"),
}

// NamedColor es un color con nombre para el editor de personaje.
type NamedColor struct {
	Name string
	C    RGB
}

var colors = []NamedColor{
	{"Original", RGB{-1, 0, 0}},
	{"Rojo", RGB{230, 60, 60}}, {"Naranja", RGB{255, 150, 50}}, {"Oro", RGB{255, 210, 70}},
	{"Verde", RGB{80, 210, 90}}, {"Turquesa", RGB{60, 210, 200}}, {"Azul", RGB{80, 130, 255}},
	{"Violeta", RGB{170, 100, 255}}, {"Rosa", RGB{255, 120, 190}}, {"Blanco", RGB{240, 240, 240}},
	{"Gris", RGB{140, 140, 150}},
}

var mirrorCh = map[rune]rune{'/': '\\', '\\': '/', '(': ')', ')': '(', '[': ']', ']': '[', '{': '}', '}': '{', '<': '>', '>': '<'}

func reverse(s string, swap bool) []rune {
	r := []rune(s)
	out := make([]rune, len(r))
	for i, c := range r {
		if m, ok := mirrorCh[c]; ok && swap {
			c = m
		}
		out[len(r)-1-i] = c
	}
	return out
}

func roleColor(role rune, prim, sec RGB) RGB {
	switch role {
	case 'P':
		return prim
	case 'S':
		return sec
	case 'K':
		return RGB{235, 195, 155}
	case 'N':
		return RGB{175, 180, 195}
	case 'E':
		return RGB{255, 255, 140}
	case 'W':
		return RGB{230, 225, 205}
	case 'D':
		return RGB{95, 95, 110}
	}
	return prim
}

// spriteCells llama put por cada celda visible; (x,y) es el centro de los pies
// y dy va de -2 a 0.
func spriteCells(si, frame int, left bool, prim, sec RGB, put func(dx, dy int, ch rune, c RGB)) {
	sp := sprites[si%len(sprites)]
	for r := 0; r < 3; r++ {
		g, m := []rune(sp.Rows[frame%2][r]), []rune(sp.Masks[frame%2][r])
		if left {
			g, m = reverse(sp.Rows[frame%2][r], true), reverse(sp.Masks[frame%2][r], false)
		}
		for i, ch := range g {
			if ch == ' ' || m[i] == ' ' {
				continue
			}
			put(i-1, r-2, ch, roleColor(m[i], prim, sec))
		}
	}
}

func padName(s string, n int) string {
	if len([]rune(s)) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len([]rune(s)))
}

// BigFrame es un cuadro de sprite grande: glifos (G) y colores (C) por fila.
type BigFrame struct{ G, C []string }

func figRows(size byte) int {
	if size == 't' {
		return 3
	}
	return len(bigSprites[size][0][0].G)
}

func figCols(size byte) int {
	if size == 't' {
		return 3
	}
	return len([]rune(bigSprites[size][0][0].G[0]))
}

func hexRGB(s string) RGB {
	v := func(i int) float64 {
		x := 0
		for _, c := range s[i : i+2] {
			x *= 16
			switch {
			case c >= '0' && c <= '9':
				x += int(c - '0')
			default:
				x += int(c-'a') + 10
			}
		}
		return float64(x)
	}
	return RGB{v(0), v(2), v(4)}
}

func hsl(c RGB) (h, s, l float64) {
	r, g, b := c.R/255, c.G/255, c.B/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

// tono dominante de cada personaje (para recolorear) y color por defecto
var domHue []float64
var defColor []RGB

func init() {
	for _, fr := range bigSprites['L'] {
		var hist [36]float64
		var sum RGB
		n := 0.0
		for _, row := range fr[0].C {
			for i := 0; i+6 <= len(row); i += 6 {
				if row[i:i+6] == "000000" {
					continue
				}
				c := hexRGB(row[i : i+6])
				h, s, _ := hsl(c)
				if s > 0.3 {
					hist[int(h*36)%36] += s
					sum.R, sum.G, sum.B = sum.R+c.R, sum.G+c.G, sum.B+c.B
					n++
				}
			}
		}
		best := 0
		for i := range hist {
			if hist[i]+hist[(i+35)%36]+hist[(i+1)%36] > hist[best]+hist[(best+35)%36]+hist[(best+1)%36] {
				best = i
			}
		}
		domHue = append(domHue, (float64(best)+0.5)/36)
		if n == 0 {
			n = 1
		}
		defColor = append(defColor, RGB{sum.R / n * 1.2, sum.G / n * 1.2, sum.B / n * 1.2})
	}
}

func recolor(c RGB, si int, prim, sec RGB) RGB {
	if prim.R < 0 && sec.R < 0 {
		return c
	}
	h, s, l := hsl(c)
	if s < 0.28 { // metal, pelo y grises de piel se quedan igual
		return c
	}
	d := math.Abs(h - domHue[si])
	if d > 0.5 {
		d = 1 - d
	}
	target := prim
	if d > 0.09 {
		target = sec
	}
	if target.R < 0 {
		return c
	}
	_, _, tl := hsl(target)
	k := (l + 0.15) / (tl + 0.15)
	return RGB{math.Min(255, target.R*k), math.Min(255, target.G*k), math.Min(255, target.B*k)}
}

// drawFig dibuja el personaje si en el tamaño dado ('t' mini, 'S', 'M', 'L');
// frame 0 = parado, 1/2 = caminando. (dx,dy) son relativos a los pies (dy<=0).
func drawFig(size byte, si, frame int, left bool, prim, sec RGB, put func(dx, dy int, ch rune, c RGB)) {
	si %= len(rosterNames)
	if size == 't' {
		f := 0
		if frame == 2 {
			f = 1
		}
		p, s := prim, sec
		if p.R < 0 {
			p = defColor[si]
		}
		if s.R < 0 {
			s = RGB{220, 220, 225}
		}
		spriteCells(si, f, left, p, s, put)
		return
	}
	fr := bigSprites[size][si][frame%3]
	rows := len(fr.G)
	for r := 0; r < rows; r++ {
		g := []rune(fr.G[r])
		cs := fr.C[r]
		cols := len(g)
		for c := 0; c < cols; c++ {
			src := c
			ch := g[c]
			if left {
				src = cols - 1 - c
				ch = g[src]
				if m, ok := mirrorCh[ch]; ok {
					ch = m
				}
			}
			if ch == ' ' {
				continue
			}
			col := recolor(hexRGB(cs[src*6:src*6+6]), si, prim, sec)
			put(c-cols/2, r-(rows-1), ch, col)
		}
	}
}
