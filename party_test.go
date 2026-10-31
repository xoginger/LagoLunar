// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"testing"
	"time"
)

func mkP(name string, x, y int) *Player {
	p := &Player{Name: name, X: x, Y: y, St: Stats{Lvl: 1, Quests: map[string]QState{}}}
	p.St.HP = p.St.maxHP()
	players[p] = true
	return p
}

func TestParty(t *testing.T) {
	setupWorld(t)
	mu.Lock()
	defer mu.Unlock()
	a, b, c, d, e := mkP("A", 500, 500), mkP("B", 510, 502), mkP("C", 505, 500), mkP("D", 504, 501), mkP("E", 503, 499)
	far := mkP("F", 900, 900)
	defer func() {
		for _, p := range []*Player{a, b, c, d, e, far} {
			delete(players, p)
		}
	}()
	if invite(a, nil) == "" || nearestOther(a) == nil {
		t.Fatal("near")
	}
	invite(a, b)
	if b.Invite == nil || acceptInvite(b) == "" || !samePartyP(a, b) || a.Party.Leader != a {
		t.Fatal("accept")
	}
	if r := invite(b, c); c.Invite != nil {
		t.Fatal("non-leader invited", r)
	}
	invite(a, c)
	acceptInvite(c)
	invite(a, d)
	acceptInvite(d)
	if r := invite(a, e); e.Invite != nil {
		t.Fatal("over max", r)
	}
	// rechazar invitación
	if len(a.Party.Members) != 4 {
		t.Fatal("members")
	}
	// invitación caducada
	kick(a, d)
	invite(a, e)
	e.Invite.At = time.Now().Add(-time.Minute)
	if acceptInvite(e); e.Party != nil {
		t.Fatal("expired invite accepted")
	}
	// xp compartida y progreso de misión
	b.St.Quests["q1"] = QState{}
	sl := newEnemy(505, 500)
	sl.T = &enemyTypes[0]
	sl.HP = 1
	enemies = append(enemies, sl)
	defer func() { enemies = enemies[:len(enemies)-1] }()
	a.Left = false
	sl.X, sl.Y = a.X+6, a.Y
	hitEnemy(a, sl, 5, RGB{})
	if a.St.XP == 0 || b.St.XP == 0 || c.St.XP == 0 || far.St.XP != 0 || b.St.Quests["q1"].Prog != 1 {
		t.Fatalf("share xp a=%d b=%d c=%d q=%v", a.St.XP, b.St.XP, c.St.XP, b.St.Quests["q1"])
	}
	// si el líder se va, pasa el liderazgo
	leaveParty(a, "dejó el equipo")
	if a.Party != nil || b.Party == nil || b.Party.Leader != b {
		t.Fatal("leader transfer")
	}
	leaveParty(c, "dejó el equipo")
	if b.Party != nil {
		t.Fatal("solo party not dissolved")
	}
	if !npcBlocked(npcs[0].X, npcs[0].Y) {
		t.Fatal("npc not solid")
	}
}
