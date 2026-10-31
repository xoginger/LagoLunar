// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestShopFlow(t *testing.T) {
	setupWorld(t)
	if len(npcs) < 3 {
		t.Fatalf("npcs: %d", len(npcs))
	}
	n := npcs[0]
	p := &Player{X: n.X + 6, Y: n.Y, Sprite: 0, St: Stats{Lvl: 1, Gold: 100}}
	p.St.HP = p.St.maxHP()
	mu.Lock()
	defer mu.Unlock()
	if npcNear(p) != n {
		t.Fatal("not near")
	}
	if r := buy(p, "hoja_lunar", n); p.St.Gold != 100 {
		t.Fatal("bought out-of-stock item:", r)
	}
	buy(p, "espada_madera", n)
	buy(p, "pocion_vida_p", n)
	if p.St.Gold != 100-30-15 || p.St.count("espada_madera") != 1 {
		t.Fatalf("gold %d inv %+v", p.St.Gold, p.St.Inv)
	}
	if r := buy(p, "cota", n); p.St.Gold != 55 || r != "No tienes suficiente oro" {
		t.Fatal("overspent", r)
	}
	a0 := p.St.atk()
	useOrEquip(p, "espada_madera")
	if p.St.atk() != a0+3 || p.St.Weapon != "espada_madera" {
		t.Fatal("equip")
	}
	if r := sell(p, "espada_madera"); p.St.Gold != 55 {
		t.Fatal("sold equipped", r)
	}
	p.St.addItem("cuero", 1)
	if r := useOrEquip(p, "cuero"); p.St.Armor != "" {
		t.Fatal("level req ignored", r)
	}
	p.St.addItem("baculo_arcano", 1)
	p.St.Lvl = 9
	if r := useOrEquip(p, "baculo_arcano"); p.St.Weapon == "baculo_arcano" {
		t.Fatal("class req ignored", r)
	}
	useOrEquip(p, "cuero")
	if p.St.maxHP() != maxHP(9)+20 || p.St.def() != 3 {
		t.Fatal("armor bonus")
	}
	useOrEquip(p, "espada_madera") // desequipar
	sell(p, "espada_madera")
	if p.St.Gold != 55+15 || p.St.count("espada_madera") != 0 {
		t.Fatalf("sell gold %d", p.St.Gold)
	}
	p.St.HP = 10
	quickPotion(p, 'h')
	if p.St.HP != 50 || p.St.count("pocion_vida_p") != 0 {
		t.Fatalf("potion hp %d", p.St.HP)
	}
}

func TestInventoryPersist(t *testing.T) {
	setupWorld(t)
	os.Setenv("DATA_DIR", t.TempDir())
	accounts = map[string]*Account{}
	loadAccounts()
	a, _ := createAccount("comprador", "secreto1")
	m := &model{acc: a, name: []rune("comprador")}
	m.join()
	mu.Lock()
	if m.p.St.count("pocion_vida_p") != 3 {
		t.Fatal("no starter kit")
	}
	m.p.St.addItem("cuero", 1)
	m.p.St.Lvl = 3
	useOrEquip(m.p, "cuero")
	mu.Unlock()
	m.leave()
	m.logout()
	accounts = map[string]*Account{}
	loadAccounts()
	b, e := login("comprador", "secreto1", "9.9.9.9")
	if b == nil {
		t.Fatal(e)
	}
	m2 := &model{acc: b, name: []rune("comprador")}
	m2.join()
	if m2.p.St.Armor != "cuero" || m2.p.St.count("cuero") != 1 || m2.p.St.count("pocion_vida_p") != 3 {
		t.Fatalf("not restored %+v", m2.p.St)
	}
	m2.leave()
	m2.logout()
}

func TestTapOneStep(t *testing.T) {
	setupWorld(t)
	os.Setenv("SPAWN", "1060,590")
	m := &model{w: 100, h: 30, mode: cm256, user: "T", name: []rune("T")}
	m.join()
	x0 := m.p.X
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	for i := 0; i < 6; i++ {
		m.Update(tick(time.Now()))
		time.Sleep(20 * time.Millisecond)
	}
	if m.p.X != x0+2 {
		t.Fatalf("tap moved %d cols", m.p.X-x0)
	}
	m.leave()
}
