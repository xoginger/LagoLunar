// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import "math/rand"

// Edificios a escala de los personajes: un personaje mide 14x17 celdas, así que
// una puerta mide unas 14 filas y una casa entre 26 y 32. Cada edificio se arma
// en un bpic y luego se estampa en el mapa: lo que no es puerta es sólido.

type bpic struct {
	w, h         int
	ch           [][]rune
	c            [][]RGB
	walk, used   [][]bool
	doorX, doorY int // centro de la puerta, relativo al bpic
}

func newPic(w, h int) *bpic {
	b := &bpic{w: w, h: h, doorX: -1}
	b.ch, b.c = make([][]rune, h), make([][]RGB, h)
	b.walk, b.used = make([][]bool, h), make([][]bool, h)
	for y := 0; y < h; y++ {
		b.ch[y], b.c[y] = make([]rune, w), make([]RGB, w)
		b.walk[y], b.used[y] = make([]bool, w), make([]bool, w)
	}
	return b
}

func (b *bpic) put(x, y int, ch rune, c RGB) {
	if x >= 0 && y >= 0 && x < b.w && y < b.h {
		b.ch[y][x], b.c[y][x], b.used[y][x], b.walk[y][x] = ch, c, true, false
	}
}

func (b *bpic) putW(x, y int, ch rune, c RGB) {
	if x >= 0 && y >= 0 && x < b.w && y < b.h {
		b.ch[y][x], b.c[y][x], b.used[y][x], b.walk[y][x] = ch, c, true, true
	}
}

func (b *bpic) text(x, y int, s string, c RGB) {
	for i, ch := range s {
		if ch != ' ' {
			b.put(x+i, y, ch, c)
		}
	}
}

// vary aclara u oscurece un color con ruido fijo por celda.
func vary(c RGB, x, y, s int, amt float64) RGB {
	k := 1 + amt*(h2(x, y, s)-0.5)
	return RGB{c.R * k, c.G * k, c.B * k}
}

// roof dibuja un tejado a dos aguas (o a cuatro si es ancho) de alto h.
func (b *bpic) roof(x0, y0, w, h int, col RGB, style int) {
	cx := x0 + w/2
	top := max(1, w/2-h*2)
	for i := 0; i < h; i++ {
		half := top + (i+1)*(w/2-top)/h
		l, r := cx-half, cx+half-1
		if w%2 == 1 {
			r++
		}
		light := 1.15 - 0.25*float64(i)/float64(h)
		for x := l; x <= r; x++ {
			var ch rune
			switch style {
			case 1: // paja
				ch = []rune{'\'', '"', '`', ','}[int(h2(x, i, 5)*4)]
				if (x+i)%5 == 0 {
					ch = '/'
				}
			case 2: // pizarra
				ch = '='
				if i%2 == 1 {
					ch = '-'
				}
				if (x+i*3)%7 == 0 {
					ch = '|'
				}
			default: // tejas
				ch = '^'
				if i%2 == 1 {
					ch = 'n'
				}
				if (x+(i%2)*2)%4 == 0 {
					ch = '|'
				}
			}
			c := vary(col, x, y0+i, 3, 0.25)
			if x > cx {
				c = RGB{c.R * 0.78, c.G * 0.78, c.B * 0.78}
			}
			c = RGB{c.R * light, c.G * light, c.B * light}
			switch {
			case x == l:
				ch = '/'
			case x == r:
				ch = '\\'
			}
			if i == 0 && x > l && x < r {
				ch = '_'
			}
			b.put(x, y0+i, ch, c)
		}
	}
	for x := x0 - 1; x <= x0+w; x++ { // alero
		b.put(x, y0+h, '=', RGB{col.R * 0.55, col.G * 0.55, col.B * 0.55})
	}
}

const (
	matPlaster = iota
	matStone
	matWood
	matAdobe
)

var wallCols = []RGB{{215, 200, 165}, {160, 155, 145}, {150, 100, 60}, {205, 150, 100}}

// walls rellena un muro con el material dado.
func (b *bpic) walls(x0, y0, w, h, mat int) {
	base := wallCols[mat]
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			var ch rune
			r := y - y0
			switch mat {
			case matStone:
				ch = '_'
				off := 0
				if r%2 == 1 {
					off = 3
				}
				if (x-x0+off)%6 == 0 {
					ch = '|'
				}
			case matWood:
				ch = '|'
				if (x-x0)%3 == 1 {
					ch = ':'
				}
			case matAdobe:
				ch = []rune{'.', ',', ':', '.', '`', '\''}[int(h2(x, y, 9)*6)]
			default:
				ch = []rune{'.', ':', '\'', '.', ','}[int(h2(x, y, 9)*5)]
				if (x-x0)%12 == 0 || r == 0 {
					ch = '#' // vigas
				}
			}
			c := vary(base, x, y, 4, 0.2)
			if ch == '#' {
				c = RGB{110, 70, 40}
			}
			if x == x0 || x == x0+w-1 {
				ch, c = '|', RGB{base.R * 0.6, base.G * 0.6, base.B * 0.6}
			}
			if r == h-1 {
				ch, c = '=', vary(RGB{120, 115, 110}, x, y, 6, 0.2)
			}
			b.put(x, y, ch, c)
		}
	}
}

func (b *bpic) window(x, y, w, h int, glass RGB) {
	frame := RGB{120, 80, 45}
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			ch, c := '#', vary(glass, x+i, y+j, 8, 0.3)
			switch {
			case (j == 0 || j == h-1) && (i == 0 || i == w-1):
				ch, c = '+', frame
			case j == 0 || j == h-1:
				ch, c = '-', frame
			case i == 0 || i == w-1:
				ch, c = '|', frame
			case i == w/2 || j == h/2:
				ch, c = '+', frame
			}
			b.put(x+i, y+j, ch, c)
		}
	}
	for i := -1; i <= w; i++ {
		b.put(x+i, y+h, '=', RGB{170, 165, 155})
	}
}

// door: arco arriba, tablones y manija. Se puede caminar por ella.
func (b *bpic) door(x, y, w, h int, col RGB) {
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			ch, c := '|', vary(col, x+i, y+j, 7, 0.25)
			if i%2 == 1 {
				ch = ':'
			}
			switch {
			case j == 0 && i == 0:
				ch = '/'
			case j == 0 && i == w-1:
				ch = '\\'
			case j == 0:
				ch = '-'
			case i == 0 || i == w-1:
				ch, c = '[', RGB{col.R * 0.6, col.G * 0.6, col.B * 0.6}
				if i == w-1 {
					ch = ']'
				}
			}
			if j == h/2+1 && i == w-2 {
				ch, c = 'o', RGB{240, 200, 80}
			}
			b.putW(x+i, y+j, ch, c)
		}
	}
	b.doorX, b.doorY = x+w/2, y+h-1
}

func (b *bpic) awning(x, y, w int, c1, c2 RGB) {
	for j := 0; j < 2; j++ {
		for i := 0; i < w; i++ {
			c := c1
			if (i/3)%2 == 1 {
				c = c2
			}
			ch := '/'
			if j == 1 {
				ch = 'v'
				if i%3 != 1 {
					ch = '\''
				}
			}
			b.put(x+i, y+j, ch, c)
		}
	}
}

var roofCols = []RGB{{180, 70, 50}, {70, 95, 160}, {190, 130, 60}, {90, 140, 70}, {140, 70, 140}, {150, 60, 50}}

// mkHouse: casa con tejado, ventanas y puerta. sign != "" la vuelve tienda.
func mkHouse(r *rand.Rand, w, h int, roof RGB, mat, rstyle int, sign string) *bpic {
	b := newPic(w+2, h)
	roofH := h * 2 / 5
	wx, ww := 2, w-2
	b.roof(1, 0, w, roofH, roof, rstyle)
	if r.Intn(2) == 0 { // chimenea
		cx := 1 + w/4 + r.Intn(w/2)
		for y := 1; y < roofH-2; y++ {
			b.put(cx, y, '|', RGB{150, 90, 70})
			b.put(cx+1, y, '#', RGB{170, 100, 80})
			b.put(cx+2, y, '|', RGB{120, 70, 55})
		}
		b.text(cx, 0, "___", RGB{120, 70, 55})
	}
	wallY := roofH + 1
	wallH := h - wallY
	b.walls(wx, wallY, ww, wallH, mat)
	dw := 7
	dh := min(14, wallH-2)
	dx := wx + ww/2 - dw/2
	b.door(dx, h-1-dh, dw, dh, RGB{130, 85, 45})
	glass := RGB{255, 215, 120}
	if r.Intn(3) == 0 {
		glass = RGB{140, 190, 230}
	}
	wy := wallY + 2
	for _, x := range []int{wx + 3, wx + ww - 10} {
		if x > wx && x+7 < dx || x > dx+dw {
			b.window(x, wy, 7, 5, glass)
		}
	}
	if sign != "" {
		b.awning(dx-4, h-1-dh-3, dw+8, RGB{220, 60, 60}, RGB{240, 230, 210})
		s := "[" + sign + "]"
		b.text(wx+ww/2-len(s)/2, wallY, s, RGB{255, 220, 90})
	}
	return b
}

// mkTemple: frontón, columnas, puerta grande y escalones.
func mkTemple(w, h int, col RGB, bell bool) *bpic {
	b := newPic(w, h)
	ped := h / 4
	cx := w / 2
	for i := 0; i < ped; i++ {
		half := (i + 1) * (w / 2) / ped
		for x := cx - half; x < cx+half; x++ {
			ch := '='
			if x == cx-half {
				ch = '/'
			} else if x == cx+half-1 {
				ch = '\\'
			} else if i == ped/2 && abs(x-cx) < 2 {
				ch = '@'
			}
			b.put(x, i, ch, vary(col, x, i, 2, 0.2))
		}
	}
	stone := RGB{215, 205, 185}
	for x := 0; x < w; x++ {
		b.put(x, ped, '=', RGB{col.R * 0.7, col.G * 0.7, col.B * 0.7})
		b.put(x, ped+1, '-', stone)
	}
	body := h - ped - 2 - 3
	for y := ped + 2; y < ped+2+body; y++ {
		for x := 1; x < w-1; x++ {
			ch, c := ':', vary(RGB{120, 110, 100}, x, y, 3, 0.2)
			if (x-2)%8 < 3 { // columnas
				ch = '|'
				if (x-2)%8 == 1 {
					ch = 'I'
				}
				c = vary(stone, x, y, 4, 0.12)
			}
			b.put(x, y, ch, c)
		}
	}
	b.door(cx-5, ped+2+body-15, 11, 15, RGB{110, 70, 40})
	for s := 0; s < 3; s++ { // escalones
		y := h - 3 + s
		for x := s * 2; x < w-s*2; x++ {
			b.put(x, y, '_', vary(stone, x, y, 5, 0.15))
		}
	}
	for y := ped + 2 + body; y < h; y++ { // los escalones del centro se caminan
		for x := cx - 5; x <= cx+5; x++ {
			b.putW(x, y, '_', stone)
		}
	}
	if bell {
		bx := w - 10
		for y := 0; y < ped+2; y++ {
			for x := bx; x < bx+8; x++ {
				ch := '#'
				if y == 0 {
					ch = '^'
				} else if y == 2 && x == bx+3 {
					ch = 'A'
				}
				b.put(x, y, ch, vary(RGB{170, 150, 130}, x, y, 6, 0.2))
			}
		}
	}
	return b
}

// mkTower: torre de piedra con almenas.
func mkTower(w, h int, roof RGB) *bpic {
	b := newPic(w, h)
	top := 0
	if roof.R >= 0 {
		top = w / 2
		b.roof(0, 0, w, top, roof, 2)
	}
	for x := 0; x < w; x++ {
		ch := '#'
		if x%3 == 2 {
			ch = '_'
		}
		b.put(x, top, ch, RGB{170, 165, 160})
	}
	b.walls(1, top+1, w-2, h-top-1, matStone)
	for y := top + 4; y < h-12; y += 7 {
		b.put(w/2, y, '|', RGB{30, 30, 40})
		b.put(w/2, y+1, '|', RGB{30, 30, 40})
	}
	return b
}

// mkCastle: torre del homenaje con dos torres y una puerta con rastrillo.
func mkCastle(w, h int, roof RGB) *bpic {
	b := newPic(w, h)
	tw := w / 5
	kt := h / 4
	b.walls(tw-1, kt, w-2*tw+2, h-kt, matStone)
	for x := tw - 1; x < w-tw+1; x++ { // almenas
		ch := '#'
		if (x/2)%2 == 1 {
			ch = '_'
		}
		b.put(x, kt-1, ch, RGB{180, 175, 170})
	}
	for _, x0 := range []int{0, w - tw} {
		t := mkTower(tw, h, roof)
		for y := 0; y < t.h; y++ {
			for x := 0; x < t.w; x++ {
				if t.used[y][x] {
					b.put(x0+x, y, t.ch[y][x], t.c[y][x])
				}
			}
		}
	}
	for i, x := range []int{w/2 - 14, w/2 + 8} { // ventanales
		b.window(x, kt+4, 7, 6, []RGB{{255, 215, 120}, {140, 190, 230}}[i])
	}
	gw, gh := 13, 16
	gx, gy := w/2-gw/2, h-gh
	for j := 0; j < gh; j++ {
		for i := 0; i < gw; i++ {
			ch, c := '+', RGB{90, 90, 100}
			if j == 0 {
				ch, c = '_', RGB{170, 165, 160}
			} else if i%3 != 0 && j%3 != 0 {
				ch, c = ' ', RGB{20, 20, 30}
			}
			b.putW(gx+i, gy+j, ch, c)
		}
	}
	b.doorX, b.doorY = w/2, h-1
	b.text(w/2-1, kt+1, "<*>", RGB{255, 220, 90})
	return b
}

func mkWell() *bpic {
	b := newPic(15, 9)
	b.roof(1, 0, 13, 3, RGB{150, 90, 50}, 0)
	for y := 4; y < 6; y++ {
		b.put(2, y, '|', RGB{120, 80, 45})
		b.put(12, y, '|', RGB{120, 80, 45})
	}
	b.put(7, 4, '|', RGB{200, 190, 160})
	b.put(7, 5, 'U', RGB{150, 110, 60})
	b.text(1, 6, "(~~~~~~~~~~~)", RGB{120, 170, 230})
	b.text(1, 7, "|_|_|_|_|_|_|", RGB{165, 160, 150})
	b.text(1, 8, "'-----------'", RGB{120, 115, 110})
	b.put(1, 6, '(', RGB{165, 160, 150})
	b.put(13, 6, ')', RGB{165, 160, 150})
	return b
}

func mkStall(r *rand.Rand, c RGB) *bpic {
	b := newPic(21, 10)
	b.awning(0, 0, 21, c, RGB{240, 235, 220})
	b.awning(0, 2, 21, c, RGB{240, 235, 220})
	for y := 4; y < 8; y++ {
		b.put(1, y, '|', RGB{130, 90, 50})
		b.put(19, y, '|', RGB{130, 90, 50})
	}
	goods := []RGB{{230, 60, 60}, {250, 170, 40}, {120, 200, 80}, {200, 120, 220}, {240, 220, 120}}
	for x := 2; x < 19; x++ {
		b.put(x, 7, []rune{'o', '@', '*', 'o', '%'}[r.Intn(5)], goods[r.Intn(len(goods))])
	}
	b.text(0, 8, "[===================]", RGB{150, 100, 55})
	b.text(0, 9, " |                 | ", RGB{110, 75, 40})
	return b
}

func mkTent(w, h int, c1, c2 RGB) *bpic {
	b := newPic(w, h)
	cx := w / 2
	for i := 0; i < h; i++ {
		half := 1 + i*(w/2)/h
		for x := cx - half; x < cx+half; x++ {
			c := c1
			if ((x-cx+40)/3)%2 == 1 {
				c = c2
			}
			ch := ':'
			switch x {
			case cx - half:
				ch = '/'
			case cx + half - 1:
				ch = '\\'
			}
			if i == 0 {
				ch = '^'
			}
			b.put(x, i, ch, vary(c, x, i, 4, 0.15))
		}
	}
	dh := min(14, h-3)
	for j := 0; j < dh; j++ { // entrada
		half := 1 + j*3/dh
		for x := cx - half; x <= cx+half; x++ {
			ch := ' '
			if x == cx-half {
				ch = '/'
			} else if x == cx+half {
				ch = '\\'
			}
			b.putW(x, h-dh+j, ch, RGB{60, 40, 30})
		}
	}
	b.doorX, b.doorY = cx, h-1
	return b
}

func mkFountain() *bpic {
	b := newPic(31, 10)
	rows := []string{
		"            _/^\\_            ",
		"           ( (o) )           ",
		"            \\ | /            ",
		"     .-------'|'-------.     ",
		"   .'  ~   ~  |  ~   ~  '.   ",
		"  (  ~   ~   ~~~   ~   ~  )  ",
		"  |'.  ~   ~   ~   ~   .'|   ",
		"  |  '-------------- -'  |   ",
		"  '._____________________.'  ",
	}
	for y, row := range rows {
		for x, ch := range row {
			if ch == ' ' {
				continue
			}
			c := RGB{185, 180, 170}
			if ch == '~' || ch == 'o' {
				c = RGB{120, 180, 255}
			}
			b.put(x, y, ch, c)
		}
	}
	return b
}

func mkPyramid(w, h int) *bpic {
	b := newPic(w, h)
	cx := w / 2
	for i := 0; i < h; i++ {
		half := 1 + i*(w/2)/h
		for x := cx - half; x < cx+half; x++ {
			ch := '#'
			if (i+(x/4))%2 == 0 {
				ch = '='
			}
			switch x {
			case cx - half:
				ch = '/'
			case cx + half - 1:
				ch = '\\'
			}
			v := 180 + 40*h2(x, i, 1)
			if x > cx {
				v *= 0.8
			}
			b.put(x, i, ch, RGB{v, v * 0.8, v * 0.45})
		}
	}
	b.door(cx-4, h-14, 9, 14, RGB{60, 45, 30})
	return b
}

// placeAt estampa el edificio con su base en la fila by, centrado en cx.
// Devuelve la posición frente a la puerta (o -1,-1 si no cupo).
func placeAt(b *bpic, cx, by int) (int, int) {
	x0, y0 := cx-b.w/2, by-b.h+1
	for y := 0; y < b.h; y++ {
		for x := 0; x < b.w; x++ {
			if b.used[y][x] && (!inb(x0+x, y0+y) || isWater(x0+x, y0+y) || y0+y < 18) {
				return -1, -1
			}
		}
	}
	for y := 0; y < b.h; y++ {
		for x := 0; x < b.w; x++ {
			if !b.used[y][x] {
				continue
			}
			wx, wy := x0+x, y0+y
			setCell(wx, wy, b.ch[y][x], b.c[y][x])
			if b.walk[y][x] {
				ttype[idx(wx, wy)] = tPave
			} else {
				ttype[idx(wx, wy)] = tWall
			}
		}
	}
	if b.doorX < 0 {
		return cx, by + 3
	}
	return x0 + b.doorX, y0 + b.doorY + 3
}

// clearRect deja un área despejada con el tipo t (sin árboles ni rocas).
func clearRect(x0, y0, x1, y1 int, t uint8) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !inb(x, y) || isWater(x, y) {
				continue
			}
			ttype[idx(x, y)] = t
			ground(x, y)
		}
	}
}

// spots guarda dónde se paran los NPCs de cada pueblo (frente a tiendas, plazas).
var spots = map[string][2]int{}

func randRoof(r *rand.Rand) RGB { return roofCols[r.Intn(len(roofCols))] }

// buildCapital: murallas con torres y cuatro puertas, palacio al norte, plaza
// con fuente y mercado al centro, y manzanas de casas en calles empedradas.
func buildCapital(r *rand.Rand) {
	cx, cy := (capX0+capX1)/2, (capY0+capY1)/2
	const wallT, wallS = 8, 6 // grosor de muralla: filas arriba/abajo, columnas a los lados
	for y := capY0 - 8; y <= capY1+8; y++ {
		for x := capX0 - 14; x <= capX1+14; x++ {
			if !inb(x, y) {
				continue
			}
			in := x >= capX0 && x <= capX1 && y >= capY0 && y <= capY1
			moat := !in && (x < capX0-8 || x > capX1+8 || y < capY0-4 || y > capY1+4)
			switch {
			case moat:
				setT(x, y, tLake)
			case !in:
				setT(x, y, tGrass)
			default:
				setT(x, y, tGrass)
			}
			ground(x, y)
		}
	}
	// muralla
	stoneC := RGB{160, 155, 148}
	for y := capY0; y <= capY1; y++ {
		for x := capX0; x <= capX1; x++ {
			top, bot := y < capY0+wallT, y > capY1-wallT
			side := x < capX0+wallS || x > capX1-wallS
			if !(top || bot || side) {
				continue
			}
			ch := '_'
			r0 := y - capY0
			if (x+(r0%2)*3)%6 == 0 {
				ch = '|'
			}
			if (top && r0 == 0) || (bot && y == capY1-wallT+1) {
				ch = '#'
				if (x/2)%2 == 1 {
					ch = '_'
				}
			}
			c := vary(stoneC, x, y, 12, 0.25)
			if (top && r0 == wallT-1) || (bot && y == capY1) {
				ch, c = '=', RGB{110, 105, 100}
			}
			setCell(x, y, ch, c)
			setT(x, y, tWall)
		}
	}
	pave := func(x0, y0, x1, y1 int) { clearRect(x0, y0, x1, y1, tPave) }
	// puertas y puentes
	pave(cx-8, capY0-8, cx+8, capY0+wallT)
	pave(cx-8, capY1-wallT, cx+8, capY1+8)
	pave(capX0-14, cy-4, capX0+wallS, cy+4)
	pave(capX1-wallS, cy-4, capX1+14, cy+4)
	for _, yy := range [][2]int{{capY0 - 8, capY0 - 1}, {capY1 + 1, capY1 + 8}} {
		for y := yy[0]; y <= yy[1]; y++ {
			for x := cx - 8; x <= cx+8; x++ {
				setT(x, y, tBridge)
				ground(x, y)
			}
		}
	}
	for _, xx := range [][2]int{{capX0 - 14, capX0 - 1}, {capX1 + 1, capX1 + 14}} {
		for x := xx[0]; x <= xx[1]; x++ {
			for y := cy - 4; y <= cy+4; y++ {
				setT(x, y, tBridge)
				ground(x, y)
			}
		}
	}
	ix0, ix1 := capX0+wallS, capX1-wallS
	iy0, iy1 := capY0+wallT, capY1-wallT
	// avenidas
	pave(cx-4, iy0, cx+3, iy1)
	pave(ix0, cy-4, ix1, cy+4)
	const lotW, lotH = 54, 46
	// la rejilla se alinea para que las avenidas caigan en calles y callejones
	by0 := cy - 37
	for by0-lotH >= iy0 {
		by0 -= lotH
	}
	lx0 := cx + 4 - lotW
	for lx0-lotW >= ix0 {
		lx0 -= lotW
	}
	plaza := [4]int{cx - 66, cy - 36, cx + 66, cy + 6}
	pal := [4]int{cx - 62, iy0 + 2, cx + 62, plaza[1] - 1}
	// manzanas: casas con la puerta hacia la calle de abajo
	free := func(x0, y0, x1, y1 int) bool {
		for _, a := range [][4]int{plaza, pal} {
			if x1 >= a[0] && x0 <= a[2] && y1 >= a[1] && y0 <= a[3] {
				return false
			}
		}
		return x1 < cx-4 || x0 > cx+3
	}
	shopDone, templeDone := false, false
	for by := by0; by+31 <= iy1; by += lotH {
		bot := by + 31 // base de las casas
		pave(ix0, bot+1, ix1, min(iy1, bot+lotH-31))
		for lx := lx0; lx+lotW-8 <= ix1; lx += lotW {
			pave(lx+lotW-8, by, lx+lotW-1, bot) // callejón
			if !free(lx, by, lx+lotW-9, bot) {
				continue
			}
			mx := lx + (lotW-8)/2
			switch {
			case !shopDone && lx > cx && bot > cy-30:
				placeAt(mkHouse(r, 42, 30, RGB{150, 40, 90}, matStone, 2, "TIENDA REAL"), mx, bot)
				spots["Mercader Real"] = [2]int{mx, bot + 6}
				shopDone = true
			case !templeDone && lx < cx && bot > cy:
				placeAt(mkTemple(44, 30, RGB{120, 200, 255}, false), mx, bot)
				spots["Sabio Ixtli"] = [2]int{mx - 12, bot + 6}
				templeDone = true
			case r.Intn(12) == 0:
				placeAt(mkWell(), mx, bot)
			default:
				w := 36 + r.Intn(7)
				placeAt(mkHouse(r, w, 26+r.Intn(5), randRoof(r), []int{matPlaster, matStone, matWood}[r.Intn(3)], r.Intn(3), ""), mx, bot)
			}
		}
	}
	// plaza y palacio van al final para que ninguna calle los borre
	pave(plaza[0], plaza[1], plaza[2], plaza[3])
	pave(pal[0], pal[1], pal[2], pal[3])
	placeAt(mkFountain(), cx, cy-14)
	for i, dx := range []int{-48, -26, 28, 50} {
		placeAt(mkStall(r, []RGB{{200, 60, 60}, {60, 120, 200}, {60, 160, 90}, {200, 150, 40}}[i]), cx+dx, cy-22)
	}
	placeAt(mkCastle(100, 52, RGB{200, 170, 60}), cx, pal[3]-10)
	for _, gx := range []int{cx - 18, cx + 18} { // torres de las puertas
		placeAt(mkTower(14, 30, RGB{80, 90, 150}), gx, capY0+wallT-1)
		placeAt(mkTower(14, 22, RGB{80, 90, 150}), gx, capY1)
	}
	for _, p := range [][2]int{{capX0 + 6, capY0 + wallT}, {capX1 - 6, capY0 + wallT}, {capX0 + 6, capY1}, {capX1 - 6, capY1}} {
		placeAt(mkTower(24, 40, RGB{80, 90, 150}), p[0], p[1])
	}
	for _, gy := range []int{cy - 6, cy + 22} {
		placeAt(mkTower(12, 18, RGB{80, 90, 150}), capX0+3, gy)
		placeAt(mkTower(12, 18, RGB{80, 90, 150}), capX1-3, gy)
	}

}

// buildVillage: plaza con pozo y puestos, casas alrededor y cercas de cultivo.
func buildVillage(r *rand.Rand, l Loc) {
	clearRect(l.X-150, l.Y-50, l.X+150, l.Y+64, tFloor)
	clearRect(l.X-80, l.Y-14, l.X+80, l.Y+12, tPave)
	clearRect(l.X-150, l.Y+48, l.X+150, l.Y+58, tPave)
	for _, x := range []int{l.X - 90, l.X + 90} {
		clearRect(x-5, l.Y+12, x+5, l.Y+48, tPave)
	}
	for i := -2; i <= 2; i++ { // fila norte, de frente a la plaza
		x := l.X + i*54
		sign := ""
		if i == 1 {
			sign = "TIENDA"
		}
		mat := matAdobe
		if r.Intn(3) == 0 {
			mat = matPlaster
		}
		fx, fy := placeAt(mkHouse(r, 40+r.Intn(6), 28, randRoof(r), mat, r.Intn(2), sign), x, l.Y-15)
		if sign != "" && fx >= 0 {
			spots["Mercader:"+l.Name] = [2]int{fx + 2, fy}
		}
	}
	for _, i := range []int{-2, -1, 1, 2} { // fila sur
		x := l.X + i*58
		placeAt(mkHouse(r, 40+r.Intn(6), 28, randRoof(r), matAdobe, 1, ""), x, l.Y+47)
	}
	placeAt(mkWell(), l.X, l.Y)
	spots["Marisol"] = [2]int{l.X - 26, l.Y + 6}
	placeAt(mkStall(r, RGB{200, 80, 60}), l.X-50, l.Y+2)
	placeAt(mkStall(r, RGB{60, 140, 200}), l.X+52, l.Y+2)
	for _, fx := range []int{l.X - 148, l.X + 110} { // cercas
		for x := fx; x < fx+38; x++ {
			for _, y := range []int{l.Y - 6, l.Y + 30} {
				if !isWater(x, y) && getT(x, y) != tPave {
					ch := '='
					if x%4 == 0 {
						ch = '+'
					}
					setCell(x, y, ch, RGB{150, 110, 70})
					setT(x, y, tWall)
				}
			}
		}
	}
}

// buildCamp: campamento de caravana con tiendas, carretas y fogata.
func buildCamp(r *rand.Rand, l Loc) {
	clearRect(l.X-90, l.Y-34, l.X+90, l.Y+14, tFloor)
	cols := [][2]RGB{{{200, 150, 80}, {230, 210, 160}}, {{170, 70, 60}, {220, 180, 120}}, {{90, 120, 170}, {210, 210, 220}}}
	for i, dx := range []int{-56, 0, 56} {
		placeAt(mkTent(32, 18, cols[i][0], cols[i][1]), l.X+dx, l.Y-6)
	}
	spots["Jefa de Caravana"] = [2]int{l.X - 28, l.Y + 2}
	spots["Mercader:"+l.Name] = [2]int{l.X + 28, l.Y + 2}
	placeAt(mkStall(r, RGB{200, 150, 40}), l.X+28, l.Y-2)
	b := newPic(9, 3) // fogata
	b.text(0, 0, "  (  )   ", RGB{255, 160, 40})
	b.text(0, 1, " ( )( )  ", RGB{255, 220, 90})
	b.text(0, 2, "/=====\\  ", RGB{120, 80, 50})
	placeAt(b, l.X, l.Y+10)
}

// buildMonastery: templo con campanario, huerto y un muro bajo.
func buildMonastery(r *rand.Rand, l Loc) {
	clearRect(l.X-80, l.Y-40, l.X+80, l.Y+16, tFloor)
	fx, fy := placeAt(mkTemple(70, 40, RGB{230, 120, 60}, true), l.X, l.Y+2)
	spots["Hermano Ascua"] = [2]int{fx - 20, fy}
	spots["Mercader:"+l.Name] = [2]int{fx + 22, fy}
	placeAt(mkHouse(r, 30, 24, RGB{170, 80, 40}, matStone, 0, ""), l.X-58, l.Y+2)
	placeAt(mkWell(), l.X+56, l.Y+2)
}

// buildLakeHamlet: la casita de la Abuela, un puesto y la tienda del capitán.
func buildLakeHamlet(r *rand.Rand) {
	top := lakeCY - lakeRY
	clearRect(lakeCX-110, top-46, lakeCX+110, top-17, tGrass)
	placeAt(mkHouse(r, 34, 26, RGB{60, 70, 130}, matWood, 1, ""), lakeCX-46, top-18)
	placeAt(mkStall(r, RGB{200, 60, 60}), lakeCX+28, top-18)
	placeAt(mkTent(26, 16, RGB{170, 40, 50}, RGB{200, 200, 210}), lakeCX+74, top-18)
	spots["Abuela Luna"] = [2]int{lakeCX - 34, top - 5}
	spots["Mercader:lago"] = [2]int{lakeCX + 28, top - 5}
	spots["Capitán Roldán"] = [2]int{lakeCX + 62, top - 4}
}
