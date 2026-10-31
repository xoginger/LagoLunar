// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

// ItemDef describe un objeto de la tienda: poción, arma o armadura.
type ItemDef struct {
	ID      string
	Name    string
	Kind    byte // h vida, m maná, w arma, a armadura
	Price   int
	Heal    int
	Mana    int
	Atk     int
	Def     int
	HP      int
	Lvl     int
	Classes []int // nil = todas
}

var itemList = []ItemDef{
	{ID: "pocion_vida_p", Name: "Poción de vida pequeña", Kind: 'h', Price: 15, Heal: 40, Lvl: 1},
	{ID: "pocion_vida", Name: "Poción de vida", Kind: 'h', Price: 40, Heal: 100, Lvl: 1},
	{ID: "pocion_vida_g", Name: "Poción de vida grande", Kind: 'h', Price: 110, Heal: 250, Lvl: 5},
	{ID: "pocion_mana_p", Name: "Poción de maná pequeña", Kind: 'm', Price: 15, Mana: 25, Lvl: 1},
	{ID: "pocion_mana", Name: "Poción de maná", Kind: 'm', Price: 40, Mana: 60, Lvl: 1},
	{ID: "pocion_mana_g", Name: "Poción de maná grande", Kind: 'm', Price: 110, Mana: 150, Lvl: 5},
	{ID: "espada_madera", Name: "Espada de madera", Kind: 'w', Price: 30, Atk: 3, Lvl: 1},
	{ID: "baculo_aprendiz", Name: "Báculo de aprendiz", Kind: 'w', Price: 80, Atk: 5, Lvl: 2, Classes: []int{1, 4}},
	{ID: "arco_corto", Name: "Arco corto", Kind: 'w', Price: 80, Atk: 5, Lvl: 2, Classes: []int{2}},
	{ID: "espada_hierro", Name: "Espada de hierro", Kind: 'w', Price: 120, Atk: 7, Lvl: 3, Classes: []int{0, 3}},
	{ID: "maza_sagrada", Name: "Maza sagrada", Kind: 'w', Price: 180, Atk: 9, Lvl: 4, Classes: []int{3}},
	{ID: "daga_sombria", Name: "Daga sombría", Kind: 'w', Price: 180, Atk: 9, Lvl: 4, Classes: []int{4}},
	{ID: "espada_acero", Name: "Espada de acero", Kind: 'w', Price: 350, Atk: 12, Lvl: 6, Classes: []int{0}},
	{ID: "baculo_arcano", Name: "Báculo arcano", Kind: 'w', Price: 380, Atk: 13, Lvl: 6, Classes: []int{1}},
	{ID: "arco_elfico", Name: "Arco élfico", Kind: 'w', Price: 360, Atk: 12, Lvl: 6, Classes: []int{2}},
	{ID: "hoja_lunar", Name: "Hoja Lunar", Kind: 'w', Price: 900, Atk: 20, Lvl: 10},
	{ID: "tunica", Name: "Túnica de tela", Kind: 'a', Price: 25, Def: 1, HP: 10, Lvl: 1},
	{ID: "cuero", Name: "Armadura de cuero", Kind: 'a', Price: 90, Def: 3, HP: 20, Lvl: 2},
	{ID: "cota", Name: "Cota de malla", Kind: 'a', Price: 220, Def: 6, HP: 40, Lvl: 4, Classes: []int{0, 2, 3}},
	{ID: "manto_arcano", Name: "Manto arcano", Kind: 'a', Price: 260, Def: 5, HP: 35, Lvl: 5, Classes: []int{1, 4}},
	{ID: "placas", Name: "Armadura de placas", Kind: 'a', Price: 520, Def: 10, HP: 70, Lvl: 7, Classes: []int{0, 3}},
	{ID: "armadura_lunar", Name: "Armadura Lunar", Kind: 'a', Price: 1200, Def: 15, HP: 120, Lvl: 10},
}

var items = map[string]*ItemDef{}

func init() {
	for i := range itemList {
		items[itemList[i].ID] = &itemList[i]
	}
}

func (d *ItemDef) sellPrice() int { return max(1, d.Price/2) }

func (d *ItemDef) desc() string {
	var p []string
	if d.Heal > 0 {
		p = append(p, fmt.Sprintf("+%d vida", d.Heal))
	}
	if d.Mana > 0 {
		p = append(p, fmt.Sprintf("+%d maná", d.Mana))
	}
	if d.Atk > 0 {
		p = append(p, fmt.Sprintf("ataque +%d", d.Atk))
	}
	if d.Def > 0 {
		p = append(p, fmt.Sprintf("defensa +%d", d.Def))
	}
	if d.HP > 0 && d.Kind == 'a' {
		p = append(p, fmt.Sprintf("vida +%d", d.HP))
	}
	return strings.Join(p, ", ")
}

func (d *ItemDef) classOK(sprite int) bool {
	if d.Classes == nil {
		return true
	}
	c := spriteClass[sprite%len(spriteClass)]
	for _, x := range d.Classes {
		if x == c {
			return true
		}
	}
	return false
}

func (d *ItemDef) classNames() string {
	if d.Classes == nil {
		return ""
	}
	var n []string
	for _, c := range d.Classes {
		n = append(n, classes[c].Name)
	}
	return strings.Join(n, "/")
}

// inventario (todo con mu tomado)
const invSlots = 24

func (s *Stats) bonus() (atk, def, hp int) {
	for _, id := range []string{s.Weapon, s.Armor} {
		if d := items[id]; d != nil {
			atk += d.Atk
			def += d.Def
			hp += d.HP
		}
	}
	return
}
func (s *Stats) maxHP() int { _, _, h := s.bonus(); return maxHP(s.Lvl) + h }
func (s *Stats) maxMP() int { return maxMP(s.Lvl) }
func (s *Stats) atk() int   { a, _, _ := s.bonus(); return attackOf(s.Lvl) + a }
func (s *Stats) def() int   { _, d, _ := s.bonus(); return d }

func (s *Stats) count(id string) int {
	for _, it := range s.Inv {
		if it.ID == id {
			return it.Qty
		}
	}
	return 0
}

func (s *Stats) addItem(id string, n int) bool {
	for i := range s.Inv {
		if s.Inv[i].ID == id {
			s.Inv[i].Qty += n
			return true
		}
	}
	if len(s.Inv) >= invSlots || items[id] == nil {
		return false
	}
	s.Inv = append(s.Inv, Item{ID: id, Qty: n})
	return true
}

func (s *Stats) removeItem(id string, n int) bool {
	for i := range s.Inv {
		if s.Inv[i].ID == id && s.Inv[i].Qty >= n {
			s.Inv[i].Qty -= n
			if s.Inv[i].Qty == 0 {
				s.Inv = append(s.Inv[:i], s.Inv[i+1:]...)
			}
			return true
		}
	}
	return false
}

// sanitizeInv quita objetos desconocidos (partidas viejas o editadas).
func (s *Stats) sanitizeInv() {
	out := s.Inv[:0]
	for _, it := range s.Inv {
		if items[it.ID] != nil && it.Qty > 0 && len(out) < invSlots {
			out = append(out, it)
		}
	}
	s.Inv = out
	if d := items[s.Weapon]; d == nil || d.Kind != 'w' {
		s.Weapon = ""
	}
	if d := items[s.Armor]; d == nil || d.Kind != 'a' {
		s.Armor = ""
	}
}

func buy(p *Player, id string, n *NPC) string {
	d := items[id]
	if d == nil || n == nil || d.Lvl > n.MaxLvl {
		return "Ese objeto no está a la venta aquí"
	}
	if p.St.Gold < d.Price {
		return "No tienes suficiente oro"
	}
	if !p.St.addItem(id, 1) {
		return "Inventario lleno"
	}
	p.St.Gold -= d.Price
	return fmt.Sprintf("Compraste %s (-%d oro)", d.Name, d.Price)
}

func sell(p *Player, id string) string {
	d := items[id]
	if d == nil {
		return "Objeto inválido"
	}
	avail := p.St.count(id)
	if p.St.Weapon == id || p.St.Armor == id {
		avail--
	}
	if avail <= 0 || !p.St.removeItem(id, 1) {
		return "No puedes vender un objeto equipado"
	}
	p.St.Gold += d.sellPrice()
	return fmt.Sprintf("Vendiste %s (+%d oro)", d.Name, d.sellPrice())
}

// useOrEquip toma la poción o equipa/desequipa arma y armadura.
func useOrEquip(p *Player, id string) string {
	d := items[id]
	if d == nil || p.St.count(id) == 0 {
		return "No tienes ese objeto"
	}
	switch d.Kind {
	case 'h':
		if p.St.HP >= p.St.maxHP() {
			return "Tu vida ya está llena"
		}
		p.St.removeItem(id, 1)
		p.heal(d.Heal)
		return "Usaste " + d.Name
	case 'm':
		if p.St.MP >= p.St.maxMP() {
			return "Tu maná ya está lleno"
		}
		p.St.removeItem(id, 1)
		p.St.MP = min(p.St.maxMP(), p.St.MP+d.Mana)
		addFx(p.X, p.Y-10, fmt.Sprintf("+%d MP", d.Mana), RGB{110, 170, 255}, 900*time.Millisecond, true)
		return "Usaste " + d.Name
	}
	slot := &p.St.Weapon
	if d.Kind == 'a' {
		slot = &p.St.Armor
	}
	if *slot == id {
		*slot = ""
		p.St.HP = min(p.St.HP, p.St.maxHP())
		return "Te quitaste " + d.Name
	}
	if p.St.Lvl < d.Lvl {
		return fmt.Sprintf("Necesitas nivel %d", d.Lvl)
	}
	if !d.classOK(p.Sprite) {
		return "Solo para " + d.classNames()
	}
	*slot = id
	p.St.HP = min(p.St.HP, p.St.maxHP())
	return "Equipaste " + d.Name
}

// quickPotion usa la poción más chica del tipo: Z vida, X maná.
func quickPotion(p *Player, kind byte) string {
	for _, d := range itemList {
		if d.Kind == kind && p.St.count(d.ID) > 0 {
			return useOrEquip(p, d.ID)
		}
	}
	if kind == 'h' {
		return "No tienes pociones de vida"
	}
	return "No tienes pociones de maná"
}

func maybeDrop(p *Player, e *Enemy) {
	if rand.Float64() > 0.15 {
		return
	}
	var pool []string
	for _, d := range itemList {
		if d.Lvl <= e.Lvl+1 && (d.Kind == 'h' || d.Kind == 'm' || rand.Intn(4) == 0) {
			pool = append(pool, d.ID)
		}
	}
	if len(pool) == 0 {
		return
	}
	id := pool[rand.Intn(len(pool))]
	if p.St.addItem(id, 1) {
		addFx(e.X, e.Y-3, "¡Obtuviste "+items[id].Name+"!", RGB{150, 230, 255}, 2*time.Second, true)
	}
}

// NPCs

// NPC es un mercader o alguien que da misiones.
type NPC struct {
	Name   string
	X, Y   int
	MaxLvl int
	ID     string
	Kind   byte // 'm' mercader, 'q' misiones
}

var npcs []*NPC

var npcArt = []string{
	"  ___  ",
	" (o o) ",
	" /|$|\\ ",
	"/_|_|_\\",
	"  / \\  ",
}

func placeNPCs() {
	npcs = nil
	rnd := rand.New(rand.NewSource(11))
	add := func(name string, x, y, lvl int) {
		for r := 0; r < 40; r++ {
			for i := 0; i < 60; i++ {
				xx, yy := x+rnd.Intn(2*r+1)-r, y+rnd.Intn(r/2+1)-r/4
				if !blocked(xx, yy) && !isWater(xx, yy) && !blocked(xx-4, yy) && !blocked(xx+4, yy) {
					npcs = append(npcs, &NPC{Name: name, X: xx, Y: yy, MaxLvl: lvl, Kind: 'm'})
					return
				}
			}
		}
	}
	at := func(key string, x, y int) (int, int) { // el lugar que dejó el generador de pueblos
		if p, ok := spots[key]; ok {
			return p[0], p[1]
		}
		return x, y
	}
	x, y := at("Mercader:lago", lakeCX+30, lakeCY-lakeRY-8)
	add("Mercader", x, y, 4)
	for _, l := range locs {
		switch l.Kind {
		case "capital":
			x, y := at("Mercader Real", l.X, l.Y+6)
			add("Mercader Real", x, y, 99)
		case "village", "pass", "monastery":
			x, y := at("Mercader:"+l.Name, l.X, l.Y+4)
			add("Mercader", x, y, 7)
		}
	}
	for _, q := range questNPCs {
		before := len(npcs)
		x, y := at(q.Name, q.X, q.Y)
		add(q.Name, x, y, 0)
		if len(npcs) > before {
			n := npcs[len(npcs)-1]
			n.Kind, n.ID = 'q', q.ID
		}
	}
}

func isNear(p *Player, n *NPC) bool { return n != nil && abs(n.X-p.X) <= 16 && abs(n.Y-p.Y) <= 4 }

// npcNear devuelve el NPC más cercano con el que se puede hablar.
func npcNear(p *Player) *NPC {
	var best *NPC
	bd := 1 << 30
	for _, n := range npcs {
		if d := abs(n.X-p.X) + 2*abs(n.Y-p.Y); isNear(p, n) && d < bd {
			best, bd = n, d
		}
	}
	return best
}

// npcHeight es el alto del sprite de un NPC en el tamaño dado.
func npcHeight(size byte) int {
	if size == 't' {
		return len(questArt)
	}
	return len(npcSprites["Mercader"][0].G)
}

func drawNPCs(layer map[int]Cell, vx, vy, vw, vh int, size byte, now time.Time) {
	for _, n := range npcs {
		if n.X < vx-10 || n.X > vx+vw+10 || n.Y < vy-2 || n.Y > vy+vh+20 {
			continue
		}
		if spr, ok := npcSprites[n.Name]; ok && size != 't' {
			// parpadeo: un cuadro corto cada pocos segundos, desfasado por NPC
			f := 0
			if (now.UnixMilli()+int64(n.X*37+n.Y*11))%3700 < 160 {
				f = 1
			}
			fr := spr[f]
			h := len(fr.G)
			for r, line := range fr.G {
				lr := []rune(line)
				for i, ch := range lr {
					if ch != ' ' {
						layer[(n.Y-h+1+r)*W+n.X-len(lr)/2+i] = Cell{ch, hexRGB(fr.C[r][i*6 : i*6+6])}
					}
				}
			}
			if n.Kind != 'q' {
				lab := []rune(n.Name + " [E]")
				for i, ch := range lab {
					layer[(n.Y-h)*W+n.X-len(lab)/2+i] = Cell{ch, RGB{255, 220, 120}}
				}
			}
			continue
		}
		art := npcArt
		if n.Kind == 'q' {
			art = questArt
		}
		h := len(art)
		for r, line := range art {
			lr := []rune(line)
			for i, ch := range lr {
				if ch == ' ' {
					continue
				}
				c := RGB{230, 190, 110}
				if n.Kind == 'q' {
					c = RGB{170, 200, 255}
				}
				if ch == '$' {
					c = RGB{255, 230, 80}
				}
				layer[(n.Y-h+1+r)*W+n.X-len(lr)/2+i] = Cell{ch, c}
			}
		}
		if n.Kind == 'q' {
			continue // la etiqueta se dibuja por jugador (! o ?)
		}
		lab := []rune(n.Name + " [E]")
		for i, ch := range lab {
			layer[(n.Y-h)*W+n.X-len(lab)/2+i] = Cell{ch, RGB{255, 220, 120}}
		}
	}
}

func shopStock(n *NPC) []string {
	var ids []string
	for _, d := range itemList {
		if d.Lvl <= n.MaxLvl {
			ids = append(ids, d.ID)
		}
	}
	return ids
}

func sortedInv(s *Stats) []Item {
	inv := append([]Item(nil), s.Inv...)
	order := map[string]int{}
	for i, d := range itemList {
		order[d.ID] = i
	}
	sort.SliceStable(inv, func(a, b int) bool { return order[inv[a].ID] < order[inv[b].ID] })
	return inv
}
