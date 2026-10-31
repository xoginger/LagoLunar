// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"strings"
)

// dialogKey se llama con mu tomado.
func (m *model) dialogKey(k string) {
	d := m.dlg
	if d == nil {
		m.win = 0
		return
	}
	switch k {
	case "esc", "q", "e", "E":
		m.win = 0
	case "left", "right", "tab", "a", "d", "up", "down", "w", "s":
		if d.mode == 'o' {
			d.sel = 1 - d.sel
		}
	case "enter", " ":
		switch d.mode {
		case 'o':
			if d.sel == 0 {
				if msg := acceptQuest(m.p, d); msg != "" {
					m.p.say2(msg)
					d.mode, d.text = 'i', "¡Cuento contigo! "+d.q.objective(QState{})
					return
				}
			}
			m.win = 0
		case 't':
			if msg := completeQuest(m.p, d); msg != "" {
				m.p.say2("Recompensa: " + msg)
				// si hay misión siguiente se ofrece de una vez
				nd := openDialog(m.p, d.npc)
				if nd.mode == 'o' || nd.mode == 't' {
					m.dlg = nd
					return
				}
			}
			m.win = 0
		default:
			m.win = 0
		}
	}
}

func (m *model) drawDialog(cv *canvas) {
	d := m.dlg
	if d == nil {
		return
	}
	bw := min(cv.w-2, 64)
	lines := wrapText(d.text, bw-6)
	extra := []string{}
	if d.q != nil && (d.mode == 'o' || d.mode == 't') {
		q := d.q
		extra = append(extra, "", "Misión: "+q.Title, "Objetivo: "+q.objective(QState{}))
		r := fmt.Sprintf("Recompensa: %d XP, %d oro", q.XP, q.Gold)
		if q.Reward != "" {
			r += ", " + items[q.Reward].Name
		}
		extra = append(extra, r)
	}
	bh := len(lines) + len(extra) + 6
	x0, y0 := (cv.w-bw)/2, max(0, cv.h-bh-2)
	bg := RGB{12, 14, 40}
	cv.box(x0, y0, bw, bh, bg, RGB{150, 190, 255})
	put := func(x, y int, s string, c RGB) {
		for i, ch := range []rune(s) {
			if xx, yy := x0+x+i, y0+y; xx < x0+bw-1 && xx < cv.w && yy < cv.h && yy >= 0 {
				cv.c[yy][xx] = TCell{ch, c, bg}
			}
		}
	}
	put(2, 0, " "+d.npc.Name+" ", RGB{255, 220, 120})
	y := 2
	for _, l := range lines {
		put(3, y, l, RGB{235, 235, 245})
		y++
	}
	for i, l := range extra {
		c := RGB{170, 200, 255}
		if i == len(extra)-1 {
			c = RGB{255, 210, 90}
		}
		put(3, y, l, c)
		y++
	}
	y++
	btn := func(x int, label string, on bool) {
		c := RGB{140, 150, 180}
		if on {
			label, c = "▸ "+label+" ◂", RGB{255, 255, 255}
		} else {
			label = "  " + label + "  "
		}
		put(x, y, label, c)
	}
	switch d.mode {
	case 'o':
		btn(bw/2-16, "Aceptar", d.sel == 0)
		btn(bw/2+2, "Rechazar", d.sel == 1)
	case 't':
		btn(bw/2-8, "Completar", true)
	default:
		btn(bw/2-6, "Cerrar", true)
	}
}

func drawQuestLog(cv *canvas, st Stats) {
	bw, bh := min(cv.w-2, 70), min(cv.h-2, 22)
	x0, y0 := (cv.w-bw)/2, (cv.h-bh)/2
	bg := RGB{10, 12, 34}
	cv.box(x0, y0, bw, bh, bg, RGB{150, 190, 255})
	put := func(x, y int, s string, c RGB) {
		for i, ch := range []rune(s) {
			if xx, yy := x0+x+i, y0+y; xx < x0+bw-1 && yy < y0+bh-1 && xx < cv.w && yy < cv.h {
				cv.c[yy][xx] = TCell{ch, c, bg}
			}
		}
	}
	put(2, 0, " DIARIO DE MISIONES ", RGB{255, 220, 120})
	done := 0
	y := 2
	for i := range questList {
		q := &questList[i]
		s, ok := st.Quests[q.ID]
		if !ok {
			continue
		}
		if s.Done {
			done++
			continue
		}
		mark, c := "•", RGB{235, 235, 245}
		if s.Ready || q.Kind == 'd' {
			mark, c = "?", RGB{140, 255, 160}
		}
		put(2, y, mark+" "+q.Title, c)
		obj := q.objective(s)
		if s.Ready {
			obj += " → vuelve con " + npcName(q.turnIn())
		}
		put(5, y+1, obj, RGB{170, 190, 220})
		where := ""
		if q.Where != "" && !s.Ready && q.Kind != 'd' {
			where = "Lugar: " + q.Where
		} else if s.Ready || q.Kind == 'd' {
			where = "Ir a: " + npcName(q.turnIn())
		}
		if where != "" {
			put(5, y+2, where+" (★ en el mapa M)", RGB{130, 150, 190})
		}
		y += 4
		if y > bh-4 {
			break
		}
	}
	if y == 2 {
		put(3, y, "No tienes misiones activas. Busca personajes con ! sobre la cabeza.", RGB{170, 170, 190})
	}
	put(2, bh-2, fmt.Sprintf("Completadas: %d/%d · L / Esc cerrar", done, len(questList)), RGB{130, 140, 180})
}

// etiquetas de NPC con ! / ? según quien mira (mu tomado al juntarlas).
type npcLabel struct {
	x, y int
	text string
	mark rune
}

func questLabels(st *Stats, ox, oy, vw, vh int, size byte) []npcLabel {
	var out []npcLabel
	for _, n := range npcs {
		if n.X < ox-20 || n.X > ox+vw+20 || n.Y < oy || n.Y > oy+vh+20 {
			continue
		}
		if n.Kind != 'q' {
			h := npcHeight(size)
			if size == 't' {
				h = len(npcArt)
			}
			out = append(out, npcLabel{n.X - ox, n.Y - h - oy, n.Name + " [E]", '$'})
			continue
		}
		mk := npcMark(st, n.ID)
		t := n.Name
		if mk != ' ' {
			t = string(mk) + " " + t + " " + string(mk)
		}
		out = append(out, npcLabel{n.X - ox, n.Y - npcHeight(size) - oy, t, mk})
	}
	return out
}

// npcTagColor es el color de la etiqueta según la marca de misión.
func npcTagColor(mark rune) RGB {
	switch mark {
	case '!':
		return RGB{255, 220, 60}
	case '?':
		return RGB{120, 255, 140}
	case '$':
		return RGB{255, 220, 120}
	}
	return RGB{170, 200, 255}
}

func objectiveMarks(st *Stats) [][2]int {
	var out [][2]int
	for i := range questList {
		q := &questList[i]
		if s, ok := st.Quests[q.ID]; ok && !s.Done {
			if x, y, ok := objectiveXY(q, s); ok {
				out = append(out, [2]int{x, y})
			}
		}
	}
	return out
}

var _ = strings.ToUpper
