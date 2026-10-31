// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"strings"
)

// colorMode es el perfil de color del terminal.
type colorMode int

const (
	cmTrue colorMode = iota
	cm256
	cm16
)

func (c colorMode) String() string { return [...]string{"truecolor", "256", "16"}[c] }

// detectMode elige el perfil de color según TERM / COLORTERM de la sesión.
// Truecolor solo si el terminal lo anuncia (Terminal.app de macOS no lo soporta).
func detectMode(term string, env []string) colorMode {
	for _, e := range env {
		if strings.HasPrefix(e, "COLORTERM=") {
			v := strings.ToLower(strings.TrimPrefix(e, "COLORTERM="))
			if v == "truecolor" || v == "24bit" {
				return cmTrue
			}
		}
	}
	t := strings.ToLower(term)
	switch {
	case strings.Contains(t, "truecolor") || strings.Contains(t, "direct"):
		return cmTrue
	case strings.Contains(t, "256"), strings.HasPrefix(t, "xterm-kitty"), strings.HasPrefix(t, "alacritty"), strings.HasPrefix(t, "tmux"), strings.HasPrefix(t, "screen"):
		return cm256
	}
	return cm16
}

var cubeLevels = [6]int{0, 95, 135, 175, 215, 255}

func nearestCube(v int) int {
	best, bd := 0, 1<<30
	for i, l := range cubeLevels {
		if d := (v - l) * (v - l); d < bd {
			best, bd = i, d
		}
	}
	return best
}

func to256(c RGB) int {
	r, g, b := cl(c.R), cl(c.G), cl(c.B)
	ri, gi, bi := nearestCube(r), nearestCube(g), nearestCube(b)
	if b > r+20 && b > 70 && bi == 0 { // los azules oscuros siguen azules, no negros
		bi = 1
	}
	cr, cg, cb := cubeLevels[ri], cubeLevels[gi], cubeLevels[bi]
	cubeIdx := 16 + 36*ri + 6*gi + bi
	dc := sq(r-cr) + sq(g-cg) + sq(b-cb)
	avg := (r + g + b) / 3
	gi2 := clamp((avg-8+5)/10, 0, 23)
	gv := 8 + gi2*10
	dg := sq(r-gv) + sq(g-gv) + sq(b-gv)
	if max(r, g, b) < 110 && max(r, g, b)-min(r, g, b) < 70 { // muy oscuro: el cubo salta a 95, mejor la escala de grises
		if avg < 4 {
			return 16
		}
		return 232 + clamp((avg-3)/10, 0, 23)
	}
	if dg < dc && max(r, g, b)-min(r, g, b) < 14 {
		return 232 + gi2
	}
	return cubeIdx
}

func sq(v int) int { return v * v }

var ansi16 = [16]RGB{{0, 0, 0}, {170, 0, 0}, {0, 170, 0}, {170, 85, 0}, {0, 0, 170}, {170, 0, 170}, {0, 170, 170}, {170, 170, 170},
	{85, 85, 85}, {255, 85, 85}, {85, 255, 85}, {255, 255, 85}, {85, 85, 255}, {255, 85, 255}, {85, 255, 255}, {255, 255, 255}}

func to16(c RGB) int {
	best, bd := 0, 1e18
	for i, a := range ansi16 {
		d := (c.R-a.R)*(c.R-a.R) + (c.G-a.G)*(c.G-a.G) + (c.B-a.B)*(c.B-a.B)
		if d < bd {
			best, bd = i, d
		}
	}
	return best
}

// esc devuelve la secuencia SGR para color de texto (bg=false) o de fondo.
func (m colorMode) esc(c RGB, bg bool) string {
	switch m {
	case cmTrue:
		k := 38
		if bg {
			k = 48
		}
		return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", k, cl(c.R), cl(c.G), cl(c.B))
	case cm256:
		k := 38
		if bg {
			k = 48
		}
		return fmt.Sprintf("\x1b[%d;5;%dm", k, to256(c))
	default:
		i := to16(c)
		if bg {
			return fmt.Sprintf("\x1b[%dm", 40+i%8) // los fondos brillantes no son confiables
		}
		if i == 0 { // nunca texto negro sobre fondo negro
			i = 8
		}
		if i >= 8 {
			return fmt.Sprintf("\x1b[%dm", 90+i-8)
		}
		return fmt.Sprintf("\x1b[%dm", 30+i)
	}
}
