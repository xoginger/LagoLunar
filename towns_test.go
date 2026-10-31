// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
)

func TestTownsWalkable(t *testing.T) {
	setupWorld(t)
	for _, n := range npcs {
		if blocked(n.X, n.Y) {
			t.Fatalf("%s está dentro de un muro en %d,%d", n.Name, n.X, n.Y)
		}
		if _, ok := npcSprites[n.Name]; !ok {
			t.Fatalf("%s no tiene sprite", n.Name)
		}
	}
	for _, k := range []string{"Mercader Real", "Sabio Ixtli", "Marisol", "Hermano Ascua", "Jefa de Caravana", "Abuela Luna", "Capitán Roldán"} {
		if _, ok := spots[k]; !ok {
			t.Errorf("sin lugar para %s", k)
		}
	}
	// las puertas de la capital y la avenida central se pueden cruzar
	cx, cy := (capX0+capX1)/2, (capY0+capY1)/2
	for y := capY0 - 6; y <= capY1+6; y++ {
		if blocked(cx, y) && y > cy+10 || blocked(cx, y) && y < capY0+12 {
			t.Fatalf("avenida norte-sur bloqueada en %d,%d", cx, y)
		}
	}
	for x := capX0 - 12; x <= capX1+12; x++ {
		if blocked(x, cy) && abs(x-cx) > 16 {
			t.Fatalf("avenida este-oeste bloqueada en %d,%d", x, cy)
		}
	}
	// una casa: la base es sólida y la puerta se puede caminar
	b := mkHouse(rand.New(rand.NewSource(1)), 40, 28, roofCols[0], matPlaster, 0, "")
	if b.doorX < 0 || !b.walk[b.doorY][b.doorX] || b.walk[b.h-1][1] {
		t.Fatal("puerta o muro mal marcados")
	}
	if dh := b.h - 1 - (b.doorY - 13); dh < 13 {
		t.Fatalf("puerta muy chica")
	}
}

// TOWNSHOT=x,y,w,h:archivo.ans go test -run TownShot dibuja una vista para capturas.
func TestTownShot(t *testing.T) {
	spec := os.Getenv("TOWNSHOT")
	if spec == "" {
		t.Skip()
	}
	setupWorld(t)
	var x, y, w, h int
	var out string
	fmt.Sscanf(strings.Replace(spec, ":", " ", 1), "%d,%d,%d,%d %s", &x, &y, &w, &h, &out)
	t.Setenv("SPAWN", fmt.Sprintf("%d,%d", x, y))
	m := &model{w: w, h: h, mode: cm256, user: "Xoco", name: []rune("Xoco"), char: 3}
	m.join()
	defer m.leave()
	fmt.Sscanf(os.Getenv("MINI"), "%d", &m.mini)
	cv := m.gameCanvas(w, h)
	os.WriteFile(out, []byte("\x1b[H"+strings.ReplaceAll(cv.String(), "\n", "\r\n")), 0o644)
}
