// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"math"
	"time"
)

const (
	partyMax   = 4
	inviteTTL  = 20 * time.Second
	shareX     = 70
	shareY     = 28
	inviteRX   = 20
	inviteRY   = 8
	partyBonus = 0.2 // +20% de XP total por cada miembro extra cerca
)

// Party es un equipo de hasta cuatro jugadores.
type Party struct {
	Leader  *Player
	Members []*Player
}

// Invite es una invitación pendiente a un equipo.
type Invite struct {
	From  *Player
	Party *Party
	At    time.Time
}

// todo lo de abajo se llama con mu tomado

func samePartyP(a, b *Player) bool {
	return a != nil && b != nil && a.Party != nil && a.Party == b.Party
}

func nearestOther(p *Player) *Player {
	var best *Player
	bd := 1 << 30
	for o := range players {
		if o == p || samePartyP(o, p) {
			continue
		}
		dx, dy := abs(o.X-p.X), abs(o.Y-p.Y)
		if dx <= inviteRX && dy <= inviteRY && dx+2*dy < bd {
			best, bd = o, dx+2*dy
		}
	}
	return best
}

func invite(from, to *Player) string {
	if to == nil || !players[to] {
		return "No hay nadie cerca para invitar"
	}
	if samePartyP(from, to) {
		return to.Name + " ya está en tu equipo"
	}
	if to.Party != nil {
		return to.Name + " ya tiene equipo"
	}
	if from.Party != nil && from.Party.Leader != from {
		return "Solo el líder puede invitar"
	}
	if from.Party != nil && len(from.Party.Members) >= partyMax {
		return "Tu equipo está lleno (máx. 4)"
	}
	if inv := to.Invite; inv != nil && time.Since(inv.At) < inviteTTL {
		return to.Name + " ya tiene una invitación pendiente"
	}
	if from.Party == nil {
		from.Party = &Party{Leader: from, Members: []*Player{from}}
	}
	to.Invite = &Invite{From: from, Party: from.Party, At: time.Now()}
	return "Invitaste a " + to.Name
}

func acceptInvite(p *Player) string {
	inv := p.Invite
	p.Invite = nil
	if inv == nil || time.Since(inv.At) > inviteTTL {
		return "La invitación expiró"
	}
	pt := inv.Party
	if !players[inv.From] || inv.From.Party != pt || p.Party != nil {
		return "La invitación ya no es válida"
	}
	if len(pt.Members) >= partyMax {
		return "El equipo está lleno"
	}
	pt.Members = append(pt.Members, p)
	p.Party = pt
	for _, mbr := range pt.Members {
		mbr.say2(p.Name + " se unió al equipo")
	}
	return "Te uniste al equipo de " + pt.Leader.Name
}

func declineInvite(p *Player) {
	if inv := p.Invite; inv != nil && players[inv.From] {
		inv.From.say2(p.Name + " rechazó la invitación")
	}
	p.Invite = nil
	cleanupSolo()
}

// cleanupSolo deshace equipos de un solo miembro sin invitaciones pendientes.
func cleanupSolo() {
	for o := range players {
		pt := o.Party
		if pt == nil || len(pt.Members) > 1 {
			continue
		}
		pending := false
		for x := range players {
			if x.Invite != nil && x.Invite.Party == pt && time.Since(x.Invite.At) < inviteTTL {
				pending = true
			}
		}
		if !pending {
			o.Party = nil
		}
	}
}

func leaveParty(p *Player, reason string) {
	pt := p.Party
	if pt == nil {
		return
	}
	p.Party = nil
	out := pt.Members[:0]
	for _, mbr := range pt.Members {
		if mbr != p {
			out = append(out, mbr)
		}
	}
	pt.Members = out
	if pt.Leader == p && len(out) > 0 {
		pt.Leader = out[0]
	}
	for _, mbr := range out {
		msg := p.Name + " " + reason
		if pt.Leader == mbr && reason != "" {
			msg += " · ahora eres el líder"
		}
		mbr.say2(msg)
	}
	if len(out) == 1 {
		out[0].Party = nil
		out[0].say2("El equipo se disolvió")
	}
	for x := range players { // invalidar invitaciones a un equipo que ya no existe
		if x.Invite != nil && x.Invite.Party == pt && len(out) <= 1 {
			x.Invite = nil
		}
	}
}

func kick(leader, target *Player) string {
	if leader.Party == nil || leader.Party.Leader != leader {
		return "Solo el líder puede expulsar"
	}
	if target == leader || target.Party != leader.Party {
		return ""
	}
	target.say2("Fuiste expulsado del equipo")
	leaveParty(target, "fue expulsado")
	return "Expulsaste a " + target.Name
}

// sharers devuelve los miembros del equipo cerca de x,y (incluido p).
func sharers(p *Player, x, y int) []*Player {
	if p.Party == nil {
		return []*Player{p}
	}
	var out []*Player
	for _, mbr := range p.Party.Members {
		if mbr == p || (players[mbr] && abs(mbr.X-x) <= shareX && abs(mbr.Y-y) <= shareY) {
			out = append(out, mbr)
		}
	}
	return out
}

func splitXP(total, n int) int {
	if n <= 1 {
		return total
	}
	return int(math.Ceil(float64(total) * (1 + partyBonus*float64(n-1)) / float64(n)))
}

func arrowTo(dx, dy int) string {
	a := math.Atan2(float64(dy), float64(dx)/2) * 180 / math.Pi
	arrows := []string{"→", "↘", "↓", "↙", "←", "↖", "↑", "↗"}
	i := int(math.Round(a/45)+8) % 8
	return arrows[i]
}

type partyRow struct {
	Name     string
	Lvl      int
	HP, Max  int
	DX, DY   int
	Leader   bool
	Me       bool
	OnScreen bool
	p        *Player
}

func partyRows(me *Player, ox, oy, vw, vh int) []partyRow {
	if me.Party == nil {
		return nil
	}
	var out []partyRow
	for _, mbr := range me.Party.Members {
		on := mbr.X >= ox && mbr.X < ox+vw && mbr.Y >= oy && mbr.Y < oy+vh
		out = append(out, partyRow{mbr.Name, mbr.St.Lvl, mbr.St.HP, mbr.MaxHP(), mbr.X - me.X, mbr.Y - me.Y, mbr == me.Party.Leader, mbr == me, on, mbr})
	}
	return out
}

func drawPartyPanel(cv *canvas, rows []partyRow, y0 int) {
	if len(rows) == 0 || cv.w < 70 {
		return
	}
	bw := 32
	x0 := cv.w - bw - 1
	bh := len(rows)*2 + 2
	cv.box(x0, y0, bw, bh, RGB{6, 14, 26}, RGB{90, 200, 210})
	cv.text(x0+2, y0, " EQUIPO (P) ", RGB{120, 230, 240})
	for i, r := range rows {
		name := r.Name
		if r.Leader {
			name = "♛ " + name
		}
		loc := ""
		if !r.Me && !r.OnScreen {
			d := int(math.Hypot(float64(r.DX)/2, float64(r.DY)))
			loc = fmt.Sprintf(" %s%dm", arrowTo(r.DX, r.DY), d)
		}
		c := RGB{140, 240, 255}
		if r.Me {
			c = RGB{255, 255, 255}
		}
		line := []rune(fmt.Sprintf("%s Nv%d%s", name, r.Lvl, loc))
		if len(line) > bw-3 {
			line = line[:bw-3]
		}
		cv.text(x0+2, y0+1+2*i, string(line), c)
		cv.text(x0+2, y0+2+2*i, "HP "+bar(r.HP, r.Max, 16)+fmt.Sprintf(" %d", max(0, r.HP)), RGB{230, 90, 90})
	}
}

func npcBlocked(x, y int) bool {
	for _, n := range npcs {
		if abs(x-n.X) <= 5 && abs(y-n.Y) <= 1 {
			return true
		}
	}
	return false
}

// partyKey se llama con mu tomado.
func (m *model) partyKey(k string) {
	pt := m.p.Party
	switch k {
	case "esc", "p", "P", "q":
		m.win = 0
		return
	case "up", "w", "k":
		m.wsel--
	case "down", "s", "j":
		m.wsel++
	case "x", "X":
		if pt != nil {
			leaveParty(m.p, "dejó el equipo")
			m.wmsg = "Saliste del equipo"
		}
	case "enter":
		if pt != nil && m.wsel >= 0 && m.wsel < len(pt.Members) {
			if msg := kick(m.p, pt.Members[m.wsel]); msg != "" {
				m.wmsg = msg
			}
		}
	}
	n := 0
	if m.p.Party != nil {
		n = len(m.p.Party.Members)
	}
	m.wsel = clamp(m.wsel, 0, max(0, n-1))
}

func (m *model) drawPartyWin(cv *canvas) {
	mu.Lock()
	var rows []partyRow
	leader := false
	if m.p != nil && m.p.Party != nil {
		rows = partyRows(m.p, -1<<20, -1<<20, 1<<21, 1<<21)
		leader = m.p.Party.Leader == m.p
	}
	mu.Unlock()
	bw, bh := min(cv.w-2, 56), 14
	x0, y0 := (cv.w-bw)/2, (cv.h-bh)/2
	cv.box(x0, y0, bw, bh, RGB{6, 14, 26}, RGB{90, 200, 210})
	cv.text(x0+2, y0, " EQUIPO ", RGB{120, 230, 240})
	if len(rows) == 0 {
		cv.text(x0+3, y0+2, "No estás en un equipo.", RGB{220, 220, 240})
		cv.text(x0+3, y0+3, "Acércate a otro jugador y pulsa T para invitarlo.", RGB{170, 180, 200})
	}
	for i, r := range rows {
		mark := "  "
		if i == m.wsel {
			mark = "▸ "
		}
		name := r.Name
		if r.Leader {
			name += " ♛ líder"
		}
		cv.text(x0+3, y0+2+i, fmt.Sprintf("%s%-22s Nv%-3d HP %d/%d", mark, name, r.Lvl, max(0, r.HP), r.Max), RGB{180, 240, 255})
	}
	if m.wmsg != "" {
		cv.text(x0+3, y0+bh-4, m.wmsg, RGB{255, 230, 140})
	}
	help := "X salir del equipo · Esc cerrar"
	if leader {
		help = "Enter expulsar · " + help
	}
	cv.text(x0+3, y0+bh-2, help, RGB{130, 140, 180})
	cv.text(x0+3, y0+bh-3, "Chat de equipo: escribe /e mensaje", RGB{130, 200, 220})
}
