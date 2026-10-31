// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func animSetup(t *testing.T, sprite int) (*model, *Enemy) {
	setupWorld(t)
	os.Setenv("SPAWN", "1060,590")
	m := &model{w: 100, h: 30, mode: cm256, user: "A", name: []rune("Heroe"), char: sprite}
	m.join()
	mu.Lock()
	p := m.p
	p.St.Lvl, p.St.MP = 10, 999
	p.Left = false
	saved := append([]*Enemy(nil), enemies...)
	var hidden []*Enemy
	for _, e := range enemies { // despejar la zona
		if !e.Dead && abs(e.X-p.X) < 120 && abs(e.Y-p.Y) < 40 {
			e.Dead = true
			hidden = append(hidden, e)
		}
	}
	e := newEnemy(p.X+24, p.Y)
	e.HP, e.Left = 1<<30, true
	enemies = append(enemies, e)
	t.Cleanup(func() {
		mu.Lock()
		enemies = saved
		for _, h := range hidden {
			h.Dead = false
		}
		fxs = nil
		mu.Unlock()
	})
	fxs = nil
	mu.Unlock()
	return m, e
}

func TestAnimProjectileFromHand(t *testing.T) {
	m, e := animSetup(t, 3) // mago
	defer m.leave()
	mu.Lock()
	special(m.p, 0) // Bola de Fuego
	hx, hy := handPos(m.p)
	var pr *Fx
	for i := range fxs {
		if fxs[i].Proj {
			pr = &fxs[i]
		}
	}
	mu.Unlock()
	if pr == nil {
		t.Fatal("no projectile")
	}
	ex, ey := bodyCenter(e)
	if pr.Y != hy || abs(pr.X-hx) > 3 || pr.X1 != ex || pr.Y1 != ey {
		t.Fatalf("proj %d,%d->%d,%d hand %d,%d body %d,%d feet %d", pr.X, pr.Y, pr.X1, pr.Y1, hx, hy, ex, ey, m.p.Y)
	}
	if hy >= m.p.Y-4 {
		t.Fatal("hand too low")
	}
	if !pr.Start.After(time.Now()) {
		t.Fatal("projectile should launch after wind-up")
	}
}

func TestAnimPoseMirror(t *testing.T) {
	for cls := 0; cls < 5; cls++ {
		for _, el := range []time.Duration{50, 150, 280} {
			l, o, ok := attackPose(cls, -1, el*time.Millisecond, white)
			if !ok || len(o) == 0 {
				t.Fatalf("cls %d el %d no pose", cls, el)
			}
			_ = l
		}
		if _, _, ok := attackPose(cls, -1, 400*time.Millisecond, white); ok {
			t.Fatal("pose should end")
		}
	}
}

// ANIMCAP=dir go test -run AnimCapture guarda cuadros ANSI para capturas y el gif.
func TestAnimCapture(t *testing.T) {
	dir := os.Getenv("ANIMCAP")
	if dir == "" {
		t.Skip()
	}
	type shot struct {
		name   string
		sprite int
		act    func(p *Player)
	}
	shots := []shot{
		{"warrior", 0, func(p *Player) { basicAttack(p) }},
		{"warrior_spin", 0, func(p *Player) { special(p, 0) }},
		{"mage", 3, func(p *Player) { special(p, 0) }},
		{"mage_nova", 3, func(p *Player) { special(p, 1) }},
		{"mage_meteor", 3, func(p *Player) { special(p, 2) }},
		{"archer", 6, func(p *Player) { special(p, 0) }},
		{"archer_rain", 6, func(p *Player) { special(p, 1) }},
		{"paladin", 1, func(p *Player) { special(p, 0) }},
		{"dark", 4, func(p *Player) { special(p, 0) }},
	}
	for _, s := range shots {
		m, _ := animSetup(t, s.sprite)
		m.gameCanvas(100, 30) // acomodar la cámara
		mu.Lock()
		s.act(m.p)
		mu.Unlock()
		t0 := time.Now()
		for i := 0; i < 22; i++ {
			for time.Since(t0) < time.Duration(i)*45*time.Millisecond {
				time.Sleep(2 * time.Millisecond)
			}
			cv := m.gameCanvas(100, 30)
			os.WriteFile(fmt.Sprintf("%s/%s_%02d.ans", dir, s.name, i), []byte("\x1b[H"+strings.ReplaceAll(cv.String(), "\n", "\r\n")), 0o644)
		}
		m.leave()
	}
}
