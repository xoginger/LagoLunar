// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"math"
	"math/rand"
	"time"
)

// fxDelay retrasa los efectos de addFx (siempre con mu tomado).
var fxDelay time.Duration

func withDelay(d time.Duration, f func()) {
	old := fxDelay
	fxDelay = d
	f()
	fxDelay = old
}

const (
	atkWind   = 100 * time.Millisecond
	atkStrike = 220 * time.Millisecond
	atkEnd    = 340 * time.Millisecond
)

func startAttack(p *Player, sk int) {
	p.St.SlashAt = time.Now()
	p.St.AtkSk = sk
}

func pdir(p *Player) int {
	if p.Left {
		return -1
	}
	return 1
}

// handPos es la mano del arma: lado hacia donde mira, a la altura del pecho.
func handPos(p *Player) (int, int) {
	return p.X + pdir(p)*(figCols('S')/2-1), p.Y - figRows('S')*45/100
}

func bodyCenter(e *Enemy) (int, int) { return e.X, e.Y - len(e.T.Art)/2 }

func travel(x0, y0, x1, y1 int, speed float64) time.Duration {
	d := math.Hypot(float64(x1-x0), float64(y1-y0)*2)
	return time.Duration(d/speed*1000) * time.Millisecond
}

func addProj(x0, y0, x1, y1 int, head string, c RGB, trail rune, tc RGB, dur time.Duration) {
	if dur < 60*time.Millisecond {
		dur = 60 * time.Millisecond
	}
	addFx(x0, y0, head, c, dur, false)
	f := &fxs[len(fxs)-1]
	f.Proj, f.X1, f.Y1, f.Trail, f.TC = true, x1, y1, trail, tc
}

// anillo de glifos corregido por la proporción de la celda
func ring(x, y, r int, g string, c RGB, dur time.Duration, n int) {
	for a := 0; a < n; a++ {
		ang := float64(a) / float64(n) * 2 * math.Pi
		addFx(x+int(math.Round(math.Cos(ang)*float64(r))), y+int(math.Round(math.Sin(ang)*float64(r)/2.2)), g, c, dur, false)
	}
}

func spark(x, y int, c RGB) {
	for _, d := range [][2]int{{0, 0}, {-1, -1}, {1, -1}, {-1, 1}, {1, 1}, {-2, 0}, {2, 0}} {
		g := "*"
		if d[0] != 0 && d[1] != 0 {
			g = "·"
		}
		if d == [2]int{0, 0} {
			g = "✸"
		}
		addFx(x+d[0], y+d[1], g, c, 220*time.Millisecond, false)
	}
}

func isRanged(cls int) bool { return cls == 1 || cls == 2 }

// impactDelay: tiempo entre la tecla y el golpe (impulso + vuelo).
func impactDelay(p *Player, e *Enemy) time.Duration {
	if !isRanged(spriteClass[p.Sprite%len(spriteClass)]) {
		return atkWind + 30*time.Millisecond
	}
	hx, hy := handPos(p)
	ex, ey := bodyCenter(e)
	return atkWind + travel(hx, hy, ex, ey, 90)
}

var white = RGB{255, 255, 255}

func basicFx(p *Player, e *Enemy, dir int) {
	cls := spriteClass[p.Sprite%len(spriteClass)]
	hx, hy := handPos(p)
	tx, ty := hx+dir*reach(), hy
	if e != nil {
		tx, ty = bodyCenter(e)
	}
	withDelay(atkWind, func() {
		switch cls {
		case 2:
			h := "→"
			if dir < 0 {
				h = "←"
			}
			addProj(hx+dir, hy, tx, ty, h, RGB{230, 210, 160}, '-', RGB{120, 100, 70}, travel(hx, hy, tx, ty, 90))
		case 1:
			addProj(hx+dir*2, hy, tx, ty, "•", RGB{160, 200, 255}, '·', RGB{80, 110, 200}, travel(hx, hy, tx, ty, 90))
		default:
			if e != nil {
				withDelay(atkWind+30*time.Millisecond, func() { spark(tx, ty, white) })
			}
		}
	})
	if e != nil && isRanged(cls) {
		withDelay(impactDelay(p, e), func() { spark(tx, ty, white) })
	}
}

func specialFx(p *Player, s Special, dir int) {
	hx, hy := handPos(p)
	cx, cy := p.X, p.Y-figRows('S')/2
	switch s.Kind {
	case 'm':
		e := nearestFacing(p, s.Range)
		tx, ty := hx+dir*6, hy
		if e != nil {
			tx, ty = bodyCenter(e)
		}
		hit := atkWind + 30*time.Millisecond
		if s.Name == "Drenar Vida" {
			fl := travel(hx, hy, tx, ty, 80)
			withDelay(atkWind, func() {
				addProj(hx+dir, hy, tx, ty, "~", s.C, '∙', RGB{90, 30, 110}, fl)
			})
			hit = atkWind + fl
		}
		withDelay(hit, func() {
			ring(tx, ty, 2, string(s.Glyph), s.C, 300*time.Millisecond, 8)
			addFx(tx, ty, "✸", white, 200*time.Millisecond, false)
		})
		if e != nil {
			d := p.damage(s.Mult)
			withDelay(hit, func() { hitEnemy(p, e, d, s.C) })
			if s.Heal > 0 {
				p.heal(int(float64(d) * s.Heal))
				if s.Name == "Drenar Vida" { // la vida regresa al lanzador
					withDelay(hit+40*time.Millisecond, func() {
						addProj(tx, ty, hx, hy, "♥", RGB{255, 90, 140}, '·', RGB{150, 40, 90}, travel(tx, ty, hx, hy, 70))
					})
				}
			}
		}
	case 'l':
		pierce := s.Name == "Disparo Perforante"
		var e *Enemy
		if !pierce {
			e = nearestFacing(p, s.Range)
		}
		tx, ty := hx+dir*s.Range, hy
		if e != nil {
			tx, ty = bodyCenter(e)
		}
		speed := 80.0
		head, trail, tc := string(s.Glyph), '•', RGB{200, 60, 20}
		switch s.Name {
		case "Flecha Certera":
			speed, trail, tc = 120, '-', RGB{140, 160, 100}
			if dir < 0 {
				head = "←"
			}
		case "Disparo Perforante":
			speed, head, trail, tc = 160, "►", '═', RGB{255, 255, 160}
			if dir < 0 {
				head = "◄"
			}
		}
		fl := travel(hx, hy, tx, ty, speed)
		withDelay(atkWind, func() {
			if s.Name == "Bola de Fuego" {
				ring(hx+dir*2, hy, 1, "*", RGB{255, 220, 80}, 150*time.Millisecond, 6)
			}
			addProj(hx+dir*2, hy, tx, ty, head, s.C, trail, tc, fl)
		})
		if e != nil {
			withDelay(atkWind+fl, func() {
				ring(tx, ty, 2, "*", s.C, 300*time.Millisecond, 8)
				ring(tx, ty, 4, "·", lerp(s.C, RGB{80, 20, 0}, 0.5), 250*time.Millisecond, 10)
				hitEnemy(p, e, p.damage(s.Mult), s.C)
			})
		}
		if pierce {
			for _, e := range enemies {
				if !e.Dead && p.facing(e, s.Range) {
					ex, ey := bodyCenter(e)
					withDelay(atkWind+travel(hx, hy, ex, hy, speed), func() {
						spark(ex, ey, s.C)
						hitEnemy(p, e, p.damage(s.Mult), s.C)
					})
				}
			}
		}
	case 'a':
		land := atkWind
		ax, ay := cx, cy
		switch s.Name {
		case "Meteoro":
			ax, ay = p.X+dir*s.Range/2, p.Y-3
			sx, sy := ax-dir*14, ay-22
			fl := 420 * time.Millisecond
			withDelay(atkWind, func() { addProj(sx, sy, ax, ay, "✹", s.C, '▒', RGB{255, 160, 40}, fl) })
			land = atkWind + fl
			withDelay(land, func() {
				for k, r := range []int{2, 5, 8, 11} {
					withDelay(land+time.Duration(k)*70*time.Millisecond, func() {
						g := []string{"█", "▓", "▒", "░"}[k]
						ring(ax, ay, r, g, lerp(RGB{255, 240, 120}, s.C, float64(k)/3), 260*time.Millisecond, 8+r*3)
					})
				}
			})
		case "Lluvia de Flechas":
			for k := 0; k < 14; k++ {
				x := p.X + rand.Intn(2*s.Range+1) - s.Range
				y := p.Y - rand.Intn(s.Range/2+1)
				d := atkWind + time.Duration(k*35)*time.Millisecond
				withDelay(d, func() { addProj(x, y-14, x, y, "↓", s.C, '¦', RGB{110, 140, 80}, 260*time.Millisecond) })
				withDelay(d+260*time.Millisecond, func() { addFx(x, y, "x", white, 160*time.Millisecond, false) })
			}
			land = atkWind + 300*time.Millisecond
		case "Tajo Giratorio":
			gl := []string{"─", "╲", "│", "╱"}
			for k := 0; k < 8; k++ {
				g := gl[k%4]
				withDelay(time.Duration(k*45)*time.Millisecond, func() {
					for r := 5; r <= s.Range/2+4; r += 3 {
						ang := float64(k)/8*2*math.Pi + float64(r)*0.2
						addFx(cx+int(math.Cos(ang)*float64(r)), cy+int(math.Sin(ang)*float64(r)/2.2), g, s.C, 120*time.Millisecond, false)
						addFx(cx-int(math.Cos(ang)*float64(r)), cy-int(math.Sin(ang)*float64(r)/2.2), g, s.C, 120*time.Millisecond, false)
					}
				})
			}
		default: // ondas que crecen: Nova de Hielo, Grito, Juicio, Llamas, Pacto
			g := string(s.Glyph)
			for k, r := range []int{s.Range / 4, s.Range / 2, s.Range * 3 / 4, s.Range} {
				gg := g
				if k%2 == 1 {
					gg = "*"
					if s.Name == "Nova de Hielo" {
						gg = "·"
					}
				}
				withDelay(atkWind+time.Duration(k*80)*time.Millisecond, func() {
					ring(cx, cy, r, gg, lerp(white, s.C, 0.3+0.2*float64(k)), 300*time.Millisecond, 10+r*2)
				})
			}
			if s.Name == "Juicio Divino" {
				for k := 0; k < 6; k++ {
					x := p.X + rand.Intn(2*s.Range+1) - s.Range
					withDelay(atkWind+time.Duration(k*50)*time.Millisecond, func() {
						addProj(x, p.Y-20, x, p.Y-2, "│", RGB{255, 255, 220}, '│', RGB{255, 240, 140}, 150*time.Millisecond)
					})
				}
			}
		}
		for _, e := range enemies {
			if !e.Dead && abs(e.X-ax) <= s.Range && abs(e.Y-p.Y) <= s.Range/2 {
				d := land + time.Duration(abs(e.X-ax))*8*time.Millisecond
				withDelay(d, func() { hitEnemy(p, e, p.damage(s.Mult), s.C) })
			}
		}
		if s.Heal > 0 {
			p.heal(int(float64(p.MaxHP()) * s.Heal))
		}
	case 'h':
		for k := 0; k < 10; k++ {
			x := p.X + rand.Intn(13) - 6
			withDelay(time.Duration(k*40)*time.Millisecond, func() {
				addFx(x, p.Y-rand.Intn(8), "+", s.C, 700*time.Millisecond, true)
			})
		}
		ring(cx, cy, 8, "✚", s.C, 300*time.Millisecond, 10)
		p.heal(int(float64(p.MaxHP()) * s.Heal))
	}
}

// poses de ataque

type ovl struct {
	dx, dy int
	ch     rune
	c      RGB
}

var (
	steel = RGB{220, 230, 245}
	wood  = RGB{170, 120, 70}
	gold  = RGB{255, 220, 90}
)

// attackPose da la inclinación del cuerpo (+ hacia adelante) y los glifos del
// arma mirando a la derecha, con dy relativo a los pies. ok=false si no ataca.
func attackPose(cls, sk int, el time.Duration, glow RGB) (lean int, o []ovl, ok bool) {
	if el < 0 || el >= atkEnd {
		return 0, nil, false
	}
	ph := 0
	if el >= atkWind {
		ph = 1
	}
	if el >= atkStrike {
		ph = 2
	}
	R := -6 // fila de la mano
	switch cls {
	case 0: // guerrero: arco de espada
		switch ph {
		case 0:
			lean = -1
			o = []ovl{{5, R - 1, '/', steel}, {4, R - 2, '/', steel}, {3, R - 3, '/', steel}, {2, R - 4, '/', steel}, {6, R, '+', wood}}
		case 1:
			lean = 1
			o = []ovl{{7, R, '═', steel}, {8, R, '═', steel}, {9, R, '═', steel}, {10, R, '>', steel},
				{9, R - 4, '.', glow}, {11, R - 3, '\\', glow}, {12, R - 1, ')', glow}, {12, R, ')', glow}, {11, R + 2, '/', glow}, {9, R + 3, '\'', glow}}
		default:
			o = []ovl{{7, R + 1, '\\', steel}, {8, R + 2, '\\', steel}, {9, R + 3, '\\', steel}}
		}
	case 2: // arquero: tensa y suelta
		bow := []ovl{{8, R - 3, '\\', wood}, {9, R - 2, ')', wood}, {9, R - 1, ')', wood}, {9, R, ')', wood}, {9, R + 1, ')', wood}, {9, R + 2, ')', wood}, {8, R + 3, '/', wood}}
		switch ph {
		case 0:
			lean = -1
			o = append(bow, ovl{7, R - 2, '\\', steel}, ovl{6, R - 1, '\\', steel}, ovl{5, R, '<', steel}, ovl{6, R + 1, '/', steel}, ovl{7, R + 2, '/', steel},
				ovl{6, R, '-', wood}, ovl{7, R, '-', wood}, ovl{8, R, '-', wood}, ovl{9, R, '-', wood}, ovl{10, R, '>', steel})
		case 1:
			o = append(bow, ovl{8, R - 2, '|', steel}, ovl{8, R - 1, '|', steel}, ovl{8, R, '|', steel}, ovl{8, R + 1, '|', steel}, ovl{8, R + 2, '|', steel}, ovl{11, R, '·', glow})
		default:
			o = bow
		}
	case 1: // mago: alza el báculo y lo empuja
		switch ph {
		case 0:
			lean = -1
			o = []ovl{{5, R + 2, '|', wood}, {5, R + 1, '|', wood}, {5, R, '|', wood}, {5, R - 1, '|', wood}, {5, R - 2, '|', wood}, {5, R - 3, '|', wood}, {5, R - 4, 'Y', wood},
				{5, R - 5, '✦', glow}, {4, R - 5, '·', glow}, {6, R - 5, '·', glow}, {5, R - 6, '·', glow}}
		case 1:
			lean = 1
			o = []ovl{{6, R - 1, '/', wood}, {7, R - 2, '/', wood}, {8, R - 3, 'Y', wood}, {9, R - 4, '✦', glow},
				{7, R, '─', glow}, {8, R, '─', glow}, {9, R, '✺', glow}}
		default:
			o = []ovl{{6, R, '/', wood}, {7, R - 1, '/', wood}, {8, R - 2, '✧', glow}}
		}
	case 3: // paladín: maza arriba con brillo sagrado
		switch ph {
		case 0:
			lean = -1
			o = []ovl{{5, R - 1, '|', wood}, {5, R - 2, '|', wood}, {5, R - 3, '|', wood}, {5, R - 4, '●', steel}, {4, R - 4, '<', steel}, {6, R - 4, '>', steel},
				{5, R - 6, '✚', gold}, {3, R - 5, '·', gold}, {7, R - 5, '·', gold}}
		case 1:
			lean = 1
			o = []ovl{{7, R, '─', wood}, {8, R, '─', wood}, {9, R, '─', wood}, {10, R, '●', steel}, {10, R - 1, '^', steel}, {10, R + 1, 'v', steel},
				{12, R - 1, '✦', gold}, {12, R + 1, '·', gold}, {11, R - 2, '·', gold}}
		default:
			o = []ovl{{7, R + 1, '\\', wood}, {8, R + 2, '●', steel}}
		}
	default: // oscuro: orbe de energía
		dark := RGB{120, 40, 180}
		switch ph {
		case 0:
			lean = -1
			o = []ovl{{5, R - 2, '(', dark}, {7, R - 2, ')', dark}, {6, R - 2, '@', glow}, {6, R - 3, '~', dark}, {6, R - 1, '~', dark}, {4, R - 4, '·', glow}, {8, R - 4, '·', glow}}
		case 1:
			lean = 1
			o = []ovl{{7, R, '~', dark}, {8, R, '§', glow}, {9, R, '~', glow}, {8, R - 1, '·', dark}, {8, R + 1, '·', dark}}
		default:
			o = []ovl{{7, R, '·', dark}}
		}
	}
	return lean, o, true
}

func atkGlow(p *Player, cls int) RGB {
	if sk := p.St.AtkSk; sk >= 0 && sk < 3 {
		return classOf(p.Sprite).Specials[sk].C
	}
	return []RGB{{255, 250, 210}, {150, 200, 255}, {230, 255, 190}, {255, 240, 150}, {200, 90, 255}}[cls]
}

// poseFor junta inclinación, arma y destello de daño de un jugador (mu tomado).
func poseFor(p *Player, now time.Time) (int, []ovl, bool) {
	cls := spriteClass[p.Sprite%len(spriteClass)]
	lean, o, _ := attackPose(cls, p.St.AtkSk, now.Sub(p.St.SlashAt), atkGlow(p, cls))
	if p.Left {
		lean = -lean
		for i := range o {
			o[i].dx = -o[i].dx
			o[i].ch = mirror(o[i].ch)
		}
	}
	hurt := !p.St.HurtAt.IsZero() && now.Sub(p.St.HurtAt) < 160*time.Millisecond
	return lean, o, hurt
}

// drawProj dibuja un proyectil en vuelo con su estela, en coordenadas de pantalla.
func drawProj(cv *canvas, f Fx, el time.Duration, ox, oy, vw, vh int) {
	k := float64(el) / float64(f.Dur)
	if k > 1 {
		k = 1
	}
	fx, fy := float64(f.X1-f.X), float64(f.Y1-f.Y)
	steps := int(math.Max(math.Abs(fx), math.Abs(fy)*2))
	if steps < 1 {
		steps = 1
	}
	set := func(px, py float64, ch rune, c RGB) {
		x, y := int(math.Round(px))-ox, int(math.Round(py))-oy
		if x >= 0 && x < vw && y >= 0 && y < vh {
			cv.c[y][x].Ch, cv.c[y][x].Fg = ch, c
		}
	}
	const tl = 7
	for i := tl; i >= 1; i-- {
		kk := k - float64(i)/float64(steps)
		if kk < 0 {
			continue
		}
		set(float64(f.X)+fx*kk, float64(f.Y)+fy*kk, f.Trail, lerp(f.TC, RGB{0, 0, 0}, float64(i)/float64(tl+2)))
	}
	for i, ch := range []rune(f.Text) {
		set(float64(f.X)+fx*k+float64(i), float64(f.Y)+fy*k, ch, f.C)
	}
}
