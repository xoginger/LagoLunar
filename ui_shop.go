// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"strings"
)

func itemName(id string) string {
	if d := items[id]; d != nil {
		return d.Name
	}
	return "—"
}

func (s *Stats) countKind(k byte) int {
	n := 0
	for _, it := range s.Inv {
		if d := items[it.ID]; d != nil && d.Kind == k {
			n += it.Qty
		}
	}
	return n
}

// winList devuelve los objetos de la ventana/pestaña actual (con mu tomado).
func (m *model) winList(st *Stats) []string {
	var ids []string
	if m.win == 1 && m.tab == 0 && m.npc != nil {
		return shopStock(m.npc)
	}
	for _, it := range sortedInv(st) {
		ids = append(ids, it.ID)
	}
	return ids
}

func (m *model) winKey(k string) {
	mu.Lock()
	defer mu.Unlock()
	if m.p == nil {
		m.win = 0
		return
	}
	if m.win == 3 {
		m.dialogKey(k)
		return
	}
	if m.win == 5 {
		m.partyKey(k)
		return
	}
	if m.win == 4 {
		if k == "esc" || k == "l" || k == "L" || k == "q" || k == "enter" {
			m.win = 0
		}
		return
	}
	list := m.winList(&m.p.St)
	switch k {
	case "esc", "q":
		m.win = 0
		return
	case "e", "E":
		if m.win == 1 {
			m.win = 0
		}
		return
	case "i", "I":
		if m.win == 2 {
			m.win = 0
		}
		return
	case "tab", "left", "right", "a", "d":
		if m.win == 1 {
			m.tab, m.wsel, m.wmsg = 1-m.tab, 0, ""
		}
		return
	case "up", "w", "k":
		m.wsel--
	case "down", "s", "j":
		m.wsel++
	case "z", "Z":
		m.wmsg = quickPotion(m.p, 'h')
	case "x", "X":
		m.wmsg = quickPotion(m.p, 'm')
	case "enter":
		if m.wsel < 0 || m.wsel >= len(list) {
			return
		}
		id := list[m.wsel]
		switch {
		case m.win == 1 && m.tab == 0:
			if !isNear(m.p, m.npc) { // se revisa en el servidor que siga junto al mercader
				m.wmsg, m.win = "Te alejaste del mercader", 0
				return
			}
			m.wmsg = buy(m.p, id, m.npc)
		case m.win == 1:
			if !isNear(m.p, m.npc) {
				m.wmsg, m.win = "Te alejaste del mercader", 0
				return
			}
			m.wmsg = sell(m.p, id)
		default:
			m.wmsg = useOrEquip(m.p, id)
		}
		list = m.winList(&m.p.St)
	}
	m.wsel = clamp(m.wsel, 0, max(0, len(list)-1))
}

func (m *model) drawWin(cv *canvas, st Stats, sprite int) {
	switch m.win {
	case 0:
		return
	case 3:
		m.drawDialog(cv)
		return
	case 4:
		drawQuestLog(cv, st)
		return
	case 5:
		m.drawPartyWin(cv)
		return
	case 6:
		m.drawAdmin(cv)
		return
	}
	list := m.winList(&st)
	bw := min(cv.w-2, 74)
	bh := min(cv.h-2, 22)
	x0, y0 := (cv.w-bw)/2, (cv.h-bh)/2
	bg := RGB{10, 12, 34}
	cv.box(x0, y0, bw, bh, bg, RGB{220, 180, 90})
	put := func(x, y int, s string, c RGB) {
		r := []rune(s)
		for i, ch := range r {
			if xx := x0 + x + i; xx < x0+bw-1 && y0+y < y0+bh-1 && y0+y >= 0 && xx >= 0 && xx < cv.w && y0+y < cv.h {
				cv.c[y0+y][xx] = TCell{ch, c, bg}
			}
		}
	}
	title := " INVENTARIO "
	if m.win == 1 && m.npc != nil {
		title = " " + strings.ToUpper(m.npc.Name) + " "
	}
	put(2, 0, title, RGB{255, 220, 120})
	gold := fmt.Sprintf(" Oro: %d ", st.Gold)
	put(bw-len([]rune(gold))-2, 0, gold, RGB{255, 210, 80})
	y := 1
	if m.win == 1 {
		tc := func(i int) RGB {
			if m.tab == i {
				return RGB{255, 255, 255}
			}
			return RGB{120, 130, 160}
		}
		c, v := "  Comprar  ", "  Vender  "
		if m.tab == 0 {
			c = "[ Comprar ]"
		} else {
			v = "[ Vender ]"
		}
		put(2, y, c, tc(0))
		put(16, y, v, tc(1))
		y += 2
	} else {
		a, d, h := st.bonus()
		put(2, y, fmt.Sprintf("Arma: %s   Armadura: %s", itemName(st.Weapon), itemName(st.Armor)), RGB{200, 220, 255})
		put(2, y+1, fmt.Sprintf("Bonos: ataque +%d  defensa +%d  vida +%d   (%d/%d espacios)", a, d, h, len(st.Inv), invSlots), RGB{150, 170, 200})
		y += 3
	}
	rows := bh - y - 4
	start := max(0, m.wsel-rows+1)
	if len(list) == 0 {
		put(4, y, "(vacío)", RGB{130, 130, 150})
	}
	for i := start; i < len(list) && i < start+rows; i++ {
		d := items[list[i]]
		if d == nil {
			continue
		}
		name := d.Name
		c := RGB{210, 215, 235}
		price := ""
		switch {
		case m.win == 1 && m.tab == 0:
			price = fmt.Sprintf("%5d oro", d.Price)
			if d.Price > st.Gold {
				c = RGB{130, 120, 130}
			}
		case m.win == 1:
			price = fmt.Sprintf("%5d oro", d.sellPrice())
		}
		if m.win != 1 || m.tab == 1 {
			name += fmt.Sprintf(" x%d", st.count(d.ID))
			if st.Weapon == d.ID || st.Armor == d.ID {
				name += " [equipado]"
				c = RGB{140, 255, 160}
			}
		}
		req := ""
		if d.Lvl > 1 {
			req = fmt.Sprintf("Nv%d", d.Lvl)
			if st.Lvl < d.Lvl {
				req = "!" + req
			}
		}
		if !d.classOK(sprite) {
			req += " " + d.classNames()
		}
		mark := "  "
		if i == m.wsel {
			mark = "▸ "
			c = RGB{255, 240, 160}
		}
		put(2, y, mark+padR(name, 34)+padR(d.desc(), 24)+price, c)
		y++
	}
	// detalle del objeto seleccionado
	if m.wsel < len(list) {
		if d := items[list[m.wsel]]; d != nil {
			det := d.desc()
			if d.Lvl > 1 {
				det += fmt.Sprintf(" · requiere nivel %d", d.Lvl)
			}
			if d.Classes != nil {
				det += " · " + d.classNames()
			}
			put(2, bh-4, det, RGB{170, 190, 220})
		}
	}
	if m.wmsg != "" {
		put(2, bh-3, m.wmsg, RGB{255, 230, 140})
	}
	help := "↑↓ elegir · Enter usar/equipar · Z/X pociones · Esc cerrar"
	if m.win == 1 {
		help = "↑↓ elegir · ←→/Tab pestaña · Enter "
		if m.tab == 0 {
			help += "comprar"
		} else {
			help += "vender (50%)"
		}
		help += " · Esc cerrar"
	}
	put(2, bh-2, help, RGB{130, 140, 180})
}
