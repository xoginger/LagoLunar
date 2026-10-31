// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"testing"
	"time"
)

var mapReady bool

func setupWorld(t testing.TB) {
	if !mapReady {
		genMap()
		initSafeZones()
		spawnEnemies()
		placeNPCs()
		mapReady = true
	}
}

func TestEnemiesTiered(t *testing.T) {
	setupWorld(t)
	if len(enemies) < 500 {
		t.Fatalf("few enemies: %d", len(enemies))
	}
	near, far := map[string]int{}, map[string]int{}
	for _, e := range enemies {
		if distLake(e.X, e.Y) < 160 {
			near[e.T.Name]++
		} else if distLake(e.X, e.Y) > 600 {
			far[e.T.Name]++
		}
	}
	t.Logf("near=%v far=%v", near, far)
	if near["Dragón joven"] > 0 || far["Slime"] > 0 {
		t.Fatal("tiers wrong")
	}
}

func TestKillLevelUp(t *testing.T) {
	setupWorld(t)
	p := &Player{Name: "T", X: 500, Y: 500, St: Stats{Lvl: 1, HP: maxHP(1), MP: maxMP(1)}}
	e := newEnemy(500, 500)
	e.X, e.Y = p.X+8, p.Y
	mu.Lock()
	old := enemies
	enemies = []*Enemy{e}
	players[p] = true
	hits := 0
	for !e.Dead && hits < 200 {
		p.St.NextAtk = time.Time{}
		basicAttack(p)
		hits++
	}
	if !e.Dead || p.St.XP == 0 && p.St.Lvl == 1 || p.St.Gold == 0 {
		t.Fatalf("kill failed dead=%v xp=%d gold=%d", e.Dead, p.St.XP, p.St.Gold)
	}
	p.gainXP(1000)
	if p.St.Lvl < 4 || p.St.HP != maxHP(p.St.Lvl) {
		t.Fatalf("lvl %d", p.St.Lvl)
	}
	// la habilidad 3 se abre en nivel 5
	p.St.Lvl, p.St.MP = 5, 100
	e2 := newEnemy(500, 500)
	e2.X, e2.Y = p.X+6, p.Y
	enemies = []*Enemy{e2}
	special(p, 2)
	if e2.HP == e2.MaxHP {
		t.Fatal("special did no damage")
	}
	// el enemigo mata al jugador y reaparece
	e2.Target, e2.NextStep, e2.NextAtk = p, time.Time{}, time.Time{}
	p.St.HP = 1
	simStep(time.Now())
	if p.St.HP != p.MaxHP() || p.St.DiedAt.IsZero() {
		t.Fatalf("death not handled hp=%d", p.St.HP)
	}
	delete(players, p)
	enemies = old
	mu.Unlock()
	t.Logf("hits to kill %s: %d, lvl now %d", e.T.Name, hits, p.St.Lvl)
}

func TestPersistStats(t *testing.T) {
	setupWorld(t)
	dir := t.TempDir()
	os.Setenv("DATA_DIR", dir)
	accounts = map[string]*Account{}
	if err := loadAccounts(); err != nil {
		t.Fatal(err)
	}
	a, e := createAccount("tester", "secreto1")
	if a == nil {
		t.Fatal(e)
	}
	m := &model{acc: a, name: []rune("tester")}
	m.join()
	mu.Lock()
	m.p.St.Lvl, m.p.St.XP, m.p.St.Gold, m.p.St.HP = 4, 33, 77, 50
	mu.Unlock()
	m.leave()
	m.logout()
	accounts = map[string]*Account{}
	loadAccounts()
	b, e := login("tester", "secreto1", "1.2.3.4")
	if b == nil {
		t.Fatal(e)
	}
	if b.Level != 4 || b.XP != 33 || b.Money != 77 || b.HP != 50 {
		t.Fatalf("not persisted: %+v", b)
	}
	m2 := &model{acc: b, name: []rune("tester")}
	m2.join()
	if m2.p.St.Lvl != 4 || m2.p.St.Gold != 77 {
		t.Fatal("not restored")
	}
	m2.leave()
	m2.logout()
}
