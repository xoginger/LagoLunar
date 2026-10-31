// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// zonas seguras

// SafeZone es un área donde los enemigos no atacan.
type SafeZone struct {
	Name string
	X, Y int
	R    float64 // en unidades de distLake: hypot(dx/2, dy)
}

var safeZones []SafeZone

func initSafeZones() {
	safeZones = []SafeZone{{"Lago Lunar", lakeCX, lakeCY, 80}}
	for _, l := range locs {
		switch l.Kind {
		case "capital":
			safeZones = append(safeZones, SafeZone{l.Name, l.X, l.Y, 120})
		case "village", "pass", "monastery":
			safeZones = append(safeZones, SafeZone{l.Name, l.X, l.Y, 45})
		}
	}
}

func safeAt(x, y int) *SafeZone {
	for i := range safeZones {
		z := &safeZones[i]
		dx, dy := float64(x-z.X)/2, float64(y-z.Y)
		if dx*dx+dy*dy < z.R*z.R {
			return z
		}
	}
	return nil
}

var questArt = []string{
	"  .^.  ",
	" (o.o) ",
	" /|~|\\ ",
	"/ |~| \\",
	"  /_\\  ",
}

// misiones
type QuestNPCDef struct {
	ID, Name string
	X, Y     int
}

var questNPCs = []QuestNPCDef{
	{"abuela", "Abuela Luna", lakeCX - 40, lakeCY - lakeRY - 9},
	{"capitan", "Capitán Roldán", lakeCX + 70, lakeCY - lakeRY - 6},
	{"sabio", "Sabio Ixtli", (capX0+capX1)/2 - 30, (capY0+capY1)/2 + 4},
	{"monje", "Hermano Ascua", 1150 - 30, 114},
	{"marisol", "Marisol", 2620 - 30, 1004},
	{"caravana", "Jefa de Caravana", 1520 - 30, 1014},
}

// QuestDef define una misión: quién la da, el objetivo y la recompensa.
type QuestDef struct {
	ID, Giver, Title string
	Text, Done       string
	Kind             byte // k matar, c recolectar, g explorar, d entregar
	Target           string
	N                int
	Item             string // recolectar: objeto; entregar: paquete
	Chance           float64
	MinLvl           int
	Prev             string
	TurnIn           string
	XP, Gold         int
	Reward           string
	Where            string // pista del lugar (también sale en el mapa)
}

var questList = []QuestDef{
	{ID: "q1", Giver: "abuela", Title: "Babosas en el sendero", Kind: 'k', Target: "Slime", N: 5, MinLvl: 1, XP: 40, Gold: 30, Reward: "pocion_vida", Where: "Lago Lunar",
		Text: "Ay, viajero... unos slimes pegajosos rondan las orillas del lago. ¿Podrías acabar con 5 de ellos?", Done: "¡Gracias! Ahora el sendero brilla otra vez bajo la luna."},
	{ID: "q2", Giver: "capitan", Title: "La manada", Kind: 'k', Target: "Lobo", N: 5, MinLvl: 2, Prev: "q1", XP: 80, Gold: 50, Reward: "tunica", Where: "Lago Lunar",
		Text: "Soldado, los lobos atacan a quien sale del lago. Caza 5 lobos y te pagaré bien.", Done: "Buen trabajo. Toma esta túnica, te hará falta."},
	{ID: "q3", Giver: "abuela", Title: "Colmillos para el amuleto", Kind: 'c', Target: "Lobo", Item: "Colmillo de lobo", N: 4, Chance: 0.5, MinLvl: 2, Prev: "q2", XP: 90, Gold: 40, Reward: "pocion_mana",
		Text: "Quiero tejer un amuleto protector. Tráeme 4 colmillos de lobo.", Done: "Con esto el amuleto estará listo. Que la luna te guíe."},
	{ID: "q4", Giver: "capitan", Title: "El santuario olvidado", Kind: 'g', Target: "Santuario de Piedra", MinLvl: 3, Prev: "q3", XP: 100, Gold: 60, Where: "Santuario de Piedra",
		Text: "Hay rumores de luces extrañas en el Santuario de Piedra, al noroeste. Ve a investigar.", Done: "¿Huesos que caminan? Esto es grave... necesito avisar a la capital."},
	{ID: "q5", Giver: "capitan", Title: "Mensaje a Lumbria", Kind: 'd', Target: "sabio", Item: "Carta del capitán", MinLvl: 3, Prev: "q4", TurnIn: "sabio", XP: 120, Gold: 80, Reward: "cuero", Where: "Ciudad de Lumbria",
		Text: "Lleva esta carta al Sabio Ixtli en la Ciudad de Lumbria, al este del lago.", Done: "Una carta de Roldán... Así que los muertos despiertan. Toma, por tu esfuerzo."},
	{ID: "q6", Giver: "sabio", Title: "Huesos inquietos", Kind: 'k', Target: "Esqueleto", N: 6, MinLvl: 4, Prev: "q5", XP: 200, Gold: 120, Where: "Templo Eterno",
		Text: "Los esqueletos se multiplican. Destruye 6 y veremos si la magia se debilita.", Done: "Bien hecho. Pero la fuente de esta magia sigue activa."},
	{ID: "q7", Giver: "sabio", Title: "Polvo de hueso", Kind: 'c', Target: "Esqueleto", Item: "Polvo de hueso", N: 5, Chance: 0.5, MinLvl: 5, Prev: "q6", XP: 240, Gold: 150, Reward: "pocion_vida_g",
		Text: "Necesito estudiar su esencia. Recoge 5 porciones de polvo de hueso.", Done: "Fascinante... esta magia viene del sur, del Templo Eterno."},
	{ID: "q8", Giver: "sabio", Title: "El Templo Eterno", Kind: 'g', Target: "Templo Eterno", MinLvl: 5, Prev: "q7", XP: 220, Gold: 120, Where: "Templo Eterno",
		Text: "Viaja al Templo Eterno y dime qué ves allí.", Done: "¿Un sello de fuego? Solo los monjes de Ascua sabrán qué significa."},
	{ID: "q9", Giver: "sabio", Title: "Pergamino sellado", Kind: 'd', Target: "monje", Item: "Pergamino sellado", MinLvl: 6, Prev: "q8", TurnIn: "monje", XP: 260, Gold: 160, Where: "Monasterio de Ascua",
		Text: "Lleva este pergamino al Hermano Ascua, en el Monasterio al norte.", Done: "El sello del fuego antiguo... hijo, has traído malas noticias."},
	{ID: "q10", Giver: "monje", Title: "Orcos en la montaña", Kind: 'k', Target: "Orco", N: 6, MinLvl: 7, Prev: "q9", XP: 400, Gold: 250, Reward: "cota",
		Text: "Los orcos sirven al fuego antiguo. Derrota a 6 de ellos.", Done: "Has demostrado valor. Esta cota de malla te protegerá."},
	{ID: "q11", Giver: "monje", Title: "El Volcán Ceniza", Kind: 'g', Target: "Volcán Ceniza", MinLvl: 8, Prev: "q10", XP: 380, Gold: 220, Where: "Volcán Ceniza",
		Text: "El corazón del fuego late en el Volcán Ceniza. Ve y obsérvalo.", Done: "Dragones... jóvenes dragones nacen del volcán."},
	{ID: "q12", Giver: "monje", Title: "Escamas de dragón", Kind: 'c', Target: "Dragón joven", Item: "Escama de dragón", N: 3, Chance: 0.6, MinLvl: 9, Prev: "q11", XP: 800, Gold: 600, Reward: "hoja_lunar", Where: "Volcán Ceniza",
		Text: "Tráeme 3 escamas de dragón y forjaré para ti la legendaria Hoja Lunar.", Done: "Aquí está: la Hoja Lunar. Eres el héroe que este continente esperaba."},
	// misiones secundarias
	{ID: "s1", Giver: "caravana", Title: "Paquete para el Alba", Kind: 'd', Target: "marisol", Item: "Paquete de especias", MinLvl: 4, TurnIn: "marisol", XP: 150, Gold: 120, Where: "Aldea del Alba",
		Text: "¿Vas al este? Entrega estas especias a Marisol en la Aldea del Alba.", Done: "¡Mis especias! Gracias, la caravana nunca llega a tiempo."},
	{ID: "s2", Giver: "marisol", Title: "Campamento orco", Kind: 'k', Target: "Orco", N: 8, MinLvl: 6, XP: 350, Gold: 200, Reward: "pocion_vida_g", Where: "Campamento Orco",
		Text: "Los orcos del campamento roban nuestras cosechas. Derrota a 8.", Done: "¡La aldea está a salvo! Toma esto."},
}

var questByID = map[string]*QuestDef{}

func init() {
	for i := range questList {
		questByID[questList[i].ID] = &questList[i]
	}
}

func (q *QuestDef) turnIn() string {
	if q.TurnIn != "" {
		return q.TurnIn
	}
	return q.Giver
}

func npcName(id string) string {
	for _, n := range questNPCs {
		if n.ID == id {
			return n.Name
		}
	}
	return id
}

// QState es el avance de un jugador en una misión.
type QState struct {
	Prog  int  `json:"prog,omitempty"`
	Ready bool `json:"ready,omitempty"`
	Done  bool `json:"done,omitempty"`
}

func copyQuests(m map[string]QState) map[string]QState {
	c := make(map[string]QState, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func (s *Stats) available(q *QuestDef) bool {
	if _, ok := s.Quests[q.ID]; ok {
		return false
	}
	if q.Prev != "" && !s.Quests[q.Prev].Done {
		return false
	}
	return s.Lvl >= q.MinLvl
}

func (s *Stats) active(q *QuestDef) bool {
	st, ok := s.Quests[q.ID]
	return ok && !st.Done
}

// npcMark: '?' para entregar, '!' misión disponible, ' ' nada.
func npcMark(s *Stats, npcID string) rune {
	for i := range questList {
		q := &questList[i]
		if s.active(q) && q.turnIn() == npcID && (s.Quests[q.ID].Ready || q.Kind == 'd') {
			return '?'
		}
	}
	for i := range questList {
		if q := &questList[i]; q.Giver == npcID && s.available(q) {
			return '!'
		}
	}
	return ' '
}

func (q *QuestDef) objective(st QState) string {
	switch q.Kind {
	case 'k':
		return fmt.Sprintf("Derrota %s: %d/%d", plural(q.Target), min(st.Prog, q.N), q.N)
	case 'c':
		return fmt.Sprintf("Consigue %s (de %s): %d/%d", q.Item, plural(q.Target), min(st.Prog, q.N), q.N)
	case 'g':
		if st.Ready {
			return "Explorado: " + q.Target
		}
		return "Explora " + q.Target
	case 'd':
		return fmt.Sprintf("Entrega %s a %s", q.Item, npcName(q.Target))
	}
	return ""
}

func plural(n string) string {
	switch n {
	case "Lobo":
		return "lobos"
	case "Slime":
		return "slimes"
	case "Esqueleto":
		return "esqueletos"
	case "Orco":
		return "orcos"
	case "Dragón joven":
		return "dragones jóvenes"
	}
	return n
}

func (p *Player) bump(q *QuestDef, label string) {
	st := p.St.Quests[q.ID]
	if st.Ready {
		return
	}
	st.Prog++
	if st.Prog >= q.N {
		st.Prog, st.Ready = q.N, true
		addFx(p.X, p.Y-14, fmt.Sprintf("¡%s completada! Vuelve con %s", q.Title, npcName(q.turnIn())), RGB{140, 255, 160}, 3*time.Second, true)
	} else {
		addFx(p.X, p.Y-14, fmt.Sprintf("%s %d/%d", label, st.Prog, q.N), RGB{180, 230, 255}, 1500*time.Millisecond, true)
	}
	p.St.Quests[q.ID] = st
}

// questKill se llama (con mu) cuando p mata a e.
func questKill(p *Player, e *Enemy) {
	for i := range questList {
		q := &questList[i]
		if !p.St.active(q) || q.Target != e.T.Name {
			continue
		}
		switch q.Kind {
		case 'k':
			p.bump(q, strings.ToUpper(plural(q.Target)[:1])+plural(q.Target)[1:])
		case 'c':
			if rand.Float64() < q.Chance {
				p.bump(q, q.Item)
			}
		}
	}
}

// questMove se llama (con mu) después de que p se mueve.
func questMove(p *Player) {
	for i := range questList {
		q := &questList[i]
		if q.Kind != 'g' || !p.St.active(q) || p.St.Quests[q.ID].Ready {
			continue
		}
		for _, l := range locs {
			if l.Name == q.Target && abs(p.X-l.X) < max(30, l.R) && abs(p.Y-l.Y) < max(14, l.R/3) {
				p.St.Quests[q.ID] = QState{Prog: 1, Ready: true}
				addFx(p.X, p.Y-14, "¡Descubriste "+l.Name+"! Vuelve con "+npcName(q.turnIn()), RGB{140, 255, 160}, 3*time.Second, true)
			}
		}
	}
}

// diálogos
type dialog struct {
	npc  *NPC
	q    *QuestDef
	mode byte // o oferta, t entregar, p progreso, i saludo
	sel  int
	text string
}

// openDialog decide qué dice el NPC (con mu tomado).
func openDialog(p *Player, n *NPC) *dialog {
	if p.St.Quests == nil {
		p.St.Quests = map[string]QState{}
	}
	for i := range questList {
		q := &questList[i]
		if p.St.active(q) && q.turnIn() == n.ID && (p.St.Quests[q.ID].Ready || q.Kind == 'd') {
			return &dialog{npc: n, q: q, mode: 't', text: q.Done}
		}
	}
	for i := range questList {
		q := &questList[i]
		if q.Giver == n.ID && p.St.available(q) {
			return &dialog{npc: n, q: q, mode: 'o', text: q.Text}
		}
	}
	for i := range questList {
		q := &questList[i]
		if q.Giver == n.ID && p.St.active(q) {
			return &dialog{npc: n, q: q, mode: 'p', text: "¿Aún no terminas? " + q.objective(p.St.Quests[q.ID])}
		}
	}
	for i := range questList {
		q := &questList[i]
		if q.Giver == n.ID && !p.St.Quests[q.ID].Done && q.Prev != "" && p.St.Quests[q.Prev].Done && p.St.Lvl < q.MinLvl {
			return &dialog{npc: n, mode: 'i', text: fmt.Sprintf("Vuelve cuando seas más fuerte (nivel %d).", q.MinLvl)}
		}
	}
	return &dialog{npc: n, mode: 'i', text: "¡Buen viaje, aventurero! Que la luna ilumine tu camino."}
}

func acceptQuest(p *Player, d *dialog) string {
	if d.q == nil || !p.St.available(d.q) || !isNear(p, d.npc) {
		return ""
	}
	p.St.Quests[d.q.ID] = QState{}
	if d.q.Kind == 'd' {
		return "Recibiste: " + d.q.Item
	}
	return "Misión aceptada: " + d.q.Title
}

func completeQuest(p *Player, d *dialog) string {
	q := d.q
	if q == nil || !p.St.active(q) || q.turnIn() != d.npc.ID || !isNear(p, d.npc) {
		return ""
	}
	if st := p.St.Quests[q.ID]; !st.Ready && q.Kind != 'd' {
		return ""
	}
	p.St.Quests[q.ID] = QState{Prog: q.N, Done: true}
	p.St.Gold += q.Gold
	msg := fmt.Sprintf("+%d XP  +%d oro", q.XP, q.Gold)
	if q.Reward != "" && p.St.addItem(q.Reward, 1) {
		msg += "  +" + items[q.Reward].Name
	}
	addFx(p.X, p.Y-14, "¡Misión cumplida! "+msg, RGB{255, 220, 90}, 3*time.Second, true)
	p.gainXP(q.XP)
	return msg
}

func questLocXY(name string) (int, int, bool) {
	for _, l := range locs {
		if l.Name == name {
			return l.X, l.Y, true
		}
	}
	for _, n := range npcs {
		if n.ID == name {
			return n.X, n.Y, true
		}
	}
	return 0, 0, false
}

// objectiveXY: a dónde ir para la misión activa.
func objectiveXY(q *QuestDef, st QState) (int, int, bool) {
	if st.Ready || q.Kind == 'd' {
		return questLocXY(q.turnIn())
	}
	if q.Where != "" {
		return questLocXY(q.Where)
	}
	return 0, 0, false
}

func wrapText(s string, w int) []string { return wrap(s, w) }
