// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"testing"
)

func qnpc(id string) *NPC {
	for _, n := range npcs {
		if n.ID == id {
			return n
		}
	}
	return nil
}

func TestSafeZones(t *testing.T) {
	setupWorld(t)
	for _, e := range enemies {
		if safeAt(e.X, e.Y) != nil {
			t.Fatalf("enemy in safe zone at %d,%d", e.X, e.Y)
		}
	}
	x, y := spawnPoint()
	if safeAt(x, y) == nil {
		t.Fatal("spawn not safe")
	}
	for _, n := range npcs {
		if safeAt(n.X, n.Y) == nil {
			t.Errorf("npc %s not in safe zone", n.Name)
		}
	}
	p := &Player{X: x, Y: y, St: Stats{Lvl: 1, HP: 50}}
	mu.Lock()
	hurtPlayer(p, 30)
	mu.Unlock()
	if p.St.HP != 50 {
		t.Fatal("damaged in safe zone")
	}
}

func TestQuestChain(t *testing.T) {
	setupWorld(t)
	for _, q := range questNPCs {
		if qnpc(q.ID) == nil {
			t.Fatalf("quest npc %s not placed", q.ID)
		}
	}
	ab := qnpc("abuela")
	p := &Player{X: ab.X + 5, Y: ab.Y, St: Stats{Lvl: 1, Quests: map[string]QState{}}}
	p.St.HP = p.St.maxHP()
	mu.Lock()
	defer mu.Unlock()
	if npcMark(&p.St, "abuela") != '!' {
		t.Fatal("no ! mark")
	}
	d := openDialog(p, ab)
	if d.mode != 'o' || d.q.ID != "q1" {
		t.Fatalf("dialog %c", d.mode)
	}
	acceptQuest(p, d)
	slime := &Enemy{T: &enemyTypes[0]}
	for i := 0; i < 5; i++ {
		questKill(p, slime)
	}
	if !p.St.Quests["q1"].Ready || npcMark(&p.St, "abuela") != '?' {
		t.Fatal("not ready")
	}
	g := p.St.Gold
	d = openDialog(p, ab)
	if d.mode != 't' || completeQuest(p, d) == "" || p.St.Gold != g+30 || p.St.count("pocion_vida") != 1 || p.St.Lvl < 2 {
		t.Fatalf("turn in failed gold=%d lvl=%d", p.St.Gold, p.St.Lvl)
	}
	if completeQuest(p, d) != "" {
		t.Fatal("double reward")
	}
	// q2..q3 rápido
	p.St.Quests["q2"] = QState{Done: true}
	p.St.Quests["q3"] = QState{Done: true}
	p.St.Lvl = 3
	cap := qnpc("capitan")
	p.X, p.Y = cap.X, cap.Y
	d = openDialog(p, cap)
	if d.q == nil || d.q.ID != "q4" {
		t.Fatal("expected q4")
	}
	acceptQuest(p, d)
	x, y, _ := questLocXY("Santuario de Piedra")
	p.X, p.Y = x, y
	questMove(p)
	if !p.St.Quests["q4"].Ready {
		t.Fatal("explore not detected")
	}
	p.X, p.Y = cap.X, cap.Y
	completeQuest(p, openDialog(p, cap))
	d = openDialog(p, cap)
	if d.q.ID != "q5" {
		t.Fatal("expected q5")
	}
	acceptQuest(p, d)
	if npcMark(&p.St, "sabio") != '?' {
		t.Fatal("deliver target not marked")
	}
	if completeQuest(p, &dialog{npc: qnpc("sabio"), q: d.q}) != "" {
		t.Fatal("delivered from far away")
	}
	sb := qnpc("sabio")
	p.X, p.Y = sb.X, sb.Y
	d = openDialog(p, sb)
	if d.mode != 't' || completeQuest(p, d) == "" || p.St.count("cuero") != 1 {
		t.Fatal("deliver failed")
	}
}

func TestQuestPersist(t *testing.T) {
	setupWorld(t)
	os.Setenv("DATA_DIR", t.TempDir())
	accounts = map[string]*Account{}
	loadAccounts()
	a, _ := createAccount("heroe", "secreto1")
	m := &model{acc: a, name: []rune("heroe")}
	m.join()
	mu.Lock()
	m.p.St.Quests["q1"] = QState{Prog: 3}
	m.p.St.Quests["q0"] = QState{Done: true}
	mu.Unlock()
	m.leave()
	m.logout()
	accounts = map[string]*Account{}
	loadAccounts()
	b, _ := login("heroe", "secreto1", "8.8.8.8")
	m2 := &model{acc: b, name: []rune("heroe")}
	m2.join()
	if m2.p.St.Quests["q1"].Prog != 3 {
		t.Fatalf("quests not restored %+v", m2.p.St.Quests)
	}
	m2.leave()
	m2.logout()
}
