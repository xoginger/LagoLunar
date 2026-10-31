// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import "sort"

// Prioridad de las etiquetas: la de menor número se coloca primero.
const (
	tagSelf = iota
	tagParty
	tagNPC
	tagPlayer
	tagEnemy
)

// tag es un nombre flotante en coordenadas de pantalla, centrado en x.
type tag struct {
	x, y int
	text string
	c    RGB
	pri  int
}

// layoutTags acomoda los nombres para que no se encimen. Cada etiqueta prueba
// su fila y si está ocupada sube o baja un poco. Si no cabe, los enemigos
// pierden el nombre (la barra de vida se queda) y el resto se dibuja igual.
// busy marca celdas que no se deben tapar, como las barras de vida.
func layoutTags(cv *canvas, tags []tag, busy [][]bool) {
	sort.SliceStable(tags, func(i, j int) bool { return tags[i].pri < tags[j].pri })
	free := func(x0, y, n int) bool {
		if y < 0 || y >= cv.h {
			return false
		}
		for x := x0 - 1; x <= x0+n; x++ {
			if x >= 0 && x < cv.w && busy[y][x] {
				return false
			}
		}
		return true
	}
	for _, t := range tags {
		r := []rune(t.text)
		x0 := t.x - len(r)/2
		if x0+len(r) < 0 || x0 >= cv.w {
			continue
		}
		y, ok := t.y, false
		for _, d := range [...]int{0, -1, 1, -2, 2} {
			if free(x0, t.y+d, len(r)) {
				y, ok = t.y+d, true
				break
			}
		}
		if !ok && t.pri == tagEnemy {
			continue
		}
		if y < 0 || y >= cv.h {
			continue
		}
		for i, ch := range r {
			if x := x0 + i; x >= 0 && x < cv.w {
				cv.c[y][x].Ch, cv.c[y][x].Fg = ch, t.c
				busy[y][x] = true
			}
		}
	}
}
