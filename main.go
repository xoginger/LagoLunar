// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

// Lago Lunar es un MMORPG en ASCII que se juega por SSH. Cada conexión es una
// sesión de bubbletea servida con wish; todos comparten un solo mundo en memoria.
package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"
)

// RGB es un color en 0-255 (float para poder mezclar).
type RGB struct{ R, G, B float64 }

// Cell es una celda del mundo: glifo y color.
type Cell struct {
	Ch rune
	C  RGB
}

var waterBlue = RGB{8, 18, 48}

// mundo compartido

// Player es un jugador dentro del mundo. X,Y son los pies del personaje.
type Player struct {
	Name      string
	MuteKey   string // cuenta (o nombre de invitado) para silenciar
	X, Y      int
	Color     RGB
	Sprite    int
	Prim, Sec RGB
	Left      bool
	Steps     int
	LastMove  time.Time
	Msg       string
	MsgAt     time.Time
	MsgParty  bool // el globo es de chat de equipo
	St        Stats
	Party     *Party
	Invite    *Invite
}

func (p *Player) frame(now time.Time) int {
	if now.Sub(p.LastMove) < 450*time.Millisecond {
		return 1 + p.Steps%2
	}
	return 0
}

// Ripple es una onda en el agua cuando alguien camina por ella.
type Ripple struct {
	X, Y  int
	Start time.Time
}

var (
	mu      sync.Mutex
	players = map[*Player]bool{}
	ripples []Ripple
	start   = time.Now()
)

var palette = []RGB{{255, 210, 90}, {255, 120, 160}, {140, 220, 255}, {200, 160, 255}, {255, 255, 255}, {255, 150, 80}}

// modelo por sesión
type tick time.Time
type phase int

const (
	phTitle phase = iota
	phHelp
	phName
	phChar
	phGame
	phLogin
	phSignup
	phWelcome
)

var menuItems = []string{"Entrar", "Crear cuenta", "Jugar sin cuenta", "Cómo jugar", "Salir"}

// model es el estado de una sesión SSH: menús, personaje y ventana abierta.
type model struct {
	p                    *Player
	user                 string
	name                 []rune
	ph                   phase
	sel                  int
	w, h                 int
	mode                 colorMode
	char, prim, sec, row int
	mini                 int
	typing               bool
	acc                  *Account
	am                   sync.Mutex
	form                 *form
	remote               string
	lastSave             time.Time
	dx, dy               int
	held                 bool
	win                  int // 0 nada, 1 tienda, 2 inventario, 3 diálogo, 4 misiones, 5 equipo, 6 admin
	tab, wsel            int
	wmsg                 string
	npc                  *NPC
	dlg                  *dialog
	rr                   *rawRenderer
	out                  *lockedW
	fbuf                 *frame
	enteredAt            time.Time
	camX, camY, camW     int
	camH                 int
	camOK, camMove       bool
	cbuf                 [][]Cell
	cvbuf                *canvas
	dirAt, stepAt        time.Time
	input                []rune
	adm                  *adminPanel
	loginAt              time.Time
}

func doTick() tea.Cmd {
	return tea.Tick(time.Second/18, func(t time.Time) tea.Msg { return tick(t) })
}
func (m *model) Init() tea.Cmd { return doTick() }

func (m *model) leave() {
	if m.p != nil {
		m.persist()
	}
	mu.Lock()
	if m.p != nil {
		leaveParty(m.p, "se desconectó")
		for o := range players {
			if o.Invite != nil && o.Invite.From == m.p {
				o.Invite = nil
			}
		}
		delete(players, m.p)
		cleanupSolo()
		m.p = nil
	}
	mu.Unlock()
}

func (m *model) join() {
	name := strings.TrimSpace(string(m.name))
	if name == "" {
		name = m.user
	}
	sx, sy := spawnPoint()
	if m.acc != nil && m.acc.HasPos && !blocked(m.acc.X, m.acc.Y) {
		sx, sy = m.acc.X, m.acc.Y
	}
	mu.Lock()
	m.p = &Player{Name: name, X: sx, Y: sy, Color: colors[m.prim].C, Sprite: m.char, Prim: colors[m.prim].C, Sec: colors[m.sec].C}
	st := Stats{Lvl: 1}
	m.am.Lock()
	if a := m.acc; a != nil {
		accMu.Lock()
		st.Lvl, st.XP, st.Gold, st.HP, st.MP = max(1, a.Level), a.XP, a.Money, a.HP, a.MP
		st.Inv, st.Weapon, st.Armor = append([]Item(nil), a.Inventory...), a.Weapon, a.Armor
		st.Quests = copyQuests(a.Quests)
		accMu.Unlock()
	}
	m.am.Unlock()
	st.sanitizeInv()
	if st.Quests == nil {
		st.Quests = map[string]QState{}
	}
	if st.Inv == nil && st.Lvl <= 1 && st.XP == 0 && st.Gold == 0 { // equipo inicial
		st.addItem("pocion_vida_p", 3)
		st.addItem("pocion_mana_p", 2)
		st.Gold = 50
	}
	if st.HP <= 0 {
		st.HP = st.maxHP()
	}
	st.HP, st.MP = min(st.HP, st.maxHP()), min(max(st.MP, 0), st.maxMP())
	if st.MP == 0 {
		st.MP = maxMP(st.Lvl)
	}
	m.p.St = st
	m.p.MuteKey = name
	if m.acc != nil {
		m.p.MuteKey = m.acc.User
	}
	players[m.p] = true
	mu.Unlock()
	m.ph = phGame
	m.lastSave = time.Now()
	m.persist()
}

// Update envuelve a update para saber cuándo se entra o sale del juego.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	was := m.ph == phGame
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.enteredAt = time.Now()
		regSize(m, ws.Width, ws.Height)
	}
	mm, cmd := m.update(msg)
	if is := m.ph == phGame; is != was {
		m.enteredAt = time.Now()
		if was && m.rr != nil { // de vuelta a pantallas de bubbletea: limpiar lo nuestro
			cmd = tea.Batch(cmd, tea.ClearScreen)
		}
	}
	return mm, cmd
}

func (m *model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tick:
		m.stepMove(time.Time(msg))
		if m.ph == phGame && m.acc != nil && time.Since(m.lastSave) > 30*time.Second {
			m.lastSave = time.Now()
			go m.persist()
		}
		return m, doTick()
	case tea.KeyMsg:
		k := msg.String()
		if k == "ctrl+c" {
			m.leave()
			m.logout()
			return m, tea.Quit
		}
		switch m.ph {
		case phTitle:
			switch k {
			case "up", "w", "k":
				m.sel = (m.sel + len(menuItems) - 1) % len(menuItems)
			case "down", "s", "j", "tab":
				m.sel = (m.sel + 1) % len(menuItems)
			case "q", "esc":
				return m, tea.Quit
			case "enter", " ":
				switch m.sel {
				case 0:
					m.ph, m.form = phLogin, newForm(false)
				case 1:
					m.ph, m.form = phSignup, newForm(true)
				case 2:
					m.ph = phName
				case 3:
					m.ph = phHelp
				case 4:
					return m, tea.Quit
				}
			}
		case phLogin, phSignup:
			m.formKey(msg, m.ph == phSignup)
		case phWelcome:
			switch k {
			case "enter":
				m.join()
			case "p", "P":
				m.ph = phChar
			case "esc":
				m.logout()
				m.ph = phTitle
			}
		case phHelp:
			if k == "enter" || k == "esc" || k == "q" || k == " " || k == "backspace" {
				m.ph = phTitle
			}
		case phName:
			switch msg.Type {
			case tea.KeyEnter:
				m.ph = phChar
			case tea.KeyEsc:
				m.ph = phTitle
			case tea.KeyBackspace:
				if len(m.name) > 0 {
					m.name = m.name[:len(m.name)-1]
				}
			case tea.KeyCtrlU:
				m.name = nil
			case tea.KeyRunes, tea.KeySpace:
				for _, r := range msg.Runes {
					if len(m.name) < 12 && r >= ' ' {
						m.name = append(m.name, r)
					}
				}
			}
		case phChar:
			step := 0
			switch k {
			case "enter":
				m.join()
				return m, nil
			case "esc":
				if m.acc != nil {
					if m.acc.Char != nil {
						m.ph = phWelcome
					} else {
						m.logout()
						m.ph = phTitle
					}
				} else {
					m.ph = phName
				}
			case "up", "w":
				m.row = (m.row + 2) % 3
			case "down", "s", "tab":
				m.row = (m.row + 1) % 3
			case "left", "a":
				step = -1
			case "right", "d":
				step = 1
			}
			if step != 0 {
				switch m.row {
				case 0:
					m.char = (m.char + step + len(sprites)) % len(sprites)
				case 1:
					m.prim = (m.prim + step + len(colors)) % len(colors)
				case 2:
					m.sec = (m.sec + step + len(colors)) % len(colors)
				}
			}
		case phGame:
			if m.p == nil {
				m.ph = phTitle
				return m, nil
			}
			if m.typing {
				switch msg.Type {
				case tea.KeyEnter:
					say(m.p, string(m.input))
					m.typing, m.input = false, nil
				case tea.KeyEsc:
					m.typing, m.input = false, nil
				case tea.KeyBackspace:
					if len(m.input) > 0 {
						m.input = m.input[:len(m.input)-1]
					}
				case tea.KeyCtrlU:
					m.input = nil
				case tea.KeyRunes, tea.KeySpace:
					for _, r := range []rune(sanitizeKeep(msg.Runes)) {
						if len(m.input) < chatMax {
							m.input = append(m.input, r)
						}
					}
				}
				return m, nil
			}
			if m.win == 6 {
				m.adminKey(msg)
				return m, nil
			}
			if m.win != 0 {
				m.winKey(k)
				return m, nil
			}
			if k == "s" || k == "S" || k == "n" || k == "N" {
				mu.Lock()
				inv := m.p.Invite
				if inv != nil && time.Since(inv.At) < inviteTTL {
					if k == "s" || k == "S" {
						m.p.say2(acceptInvite(m.p))
					} else {
						declineInvite(m.p)
						m.p.say2("Rechazaste la invitación")
					}
					mu.Unlock()
					return m, nil
				}
				mu.Unlock()
			}
			if msg.Type == tea.KeySpace || k == " " {
				m.typing, m.input, m.dx, m.dy = true, nil, 0, 0
				return m, nil
			}
			dx, dy := 0, 0
			switch k {
			case "q":
				m.leave()
				m.logout()
				return m, tea.Quit
			case "esc":
				m.leave()
				m.ph = phTitle
				if m.acc != nil {
					m.ph = phWelcome
				}
				return m, nil
			case "o", "O", "ctrl+a":
				m.openAdmin() // no hace nada si la cuenta no es admin
				return m, nil
			case "m", "M":
				m.mini = (m.mini + 1) % 3
				return m, nil
			case "e", "E":
				mu.Lock()
				n := npcNear(m.p)
				mu.Unlock()
				if n != nil && n.Kind == 'q' {
					mu.Lock()
					m.dlg = openDialog(m.p, n)
					mu.Unlock()
					m.win, m.npc, m.wmsg = 3, n, ""
					m.dx, m.dy = 0, 0
				} else if n != nil {
					m.win, m.npc, m.tab, m.wsel, m.wmsg = 1, n, 0, 0, ""
					m.dx, m.dy = 0, 0
				}
				return m, nil
			case "t", "T":
				mu.Lock()
				m.p.say2(invite(m.p, nearestOther(m.p)))
				mu.Unlock()
				return m, nil
			case "p", "P":
				m.win, m.wsel, m.wmsg = 5, 0, ""
				m.dx, m.dy = 0, 0
				return m, nil
			case "l", "L":
				m.win, m.wsel, m.wmsg = 4, 0, ""
				m.dx, m.dy = 0, 0
				return m, nil
			case "i", "I":
				m.win, m.wsel, m.wmsg = 2, 0, ""
				m.dx, m.dy = 0, 0
				return m, nil
			case "z", "Z", "x", "X":
				kind := byte('h')
				if k == "x" || k == "X" {
					kind = 'm'
				}
				mu.Lock()
				m.p.say2(quickPotion(m.p, kind))
				mu.Unlock()
				return m, nil
			case "f", "F", "j", "J":
				mu.Lock()
				basicAttack(m.p)
				mu.Unlock()
				return m, nil
			case "1", "2", "3":
				mu.Lock()
				special(m.p, int(k[0]-'1'))
				mu.Unlock()
				return m, nil
			case "up", "w":
				dy = -1
			case "down", "s":
				dy = 1
			case "left", "a":
				dx = -1
			case "right", "d":
				dx = 1
			}
			if dx != 0 || dy != 0 {
				now := time.Now()
				fresh := m.dx != dx || m.dy != dy || now.Sub(m.dirAt) > holdWindow
				m.dx, m.dy, m.dirAt = dx, dy, now
				m.held = !fresh // movimiento continuo solo cuando llega la repetición de tecla
				if fresh {      // toque: un paso, de inmediato
					m.step(now)
				}
			}
			return m, nil
		}
	}
	return m, nil
}

func onlineCount() int {
	mu.Lock()
	defer mu.Unlock()
	return len(players)
}

// View dibuja menús con bubbletea; dentro del juego devuelve un texto fijo
// porque los cuadros los manda rawRenderer.
func (m *model) View() string {
	vw, vh := m.w, m.h
	if vw <= 0 || vh <= 0 {
		vw, vh = 80, 24
	}
	if m.ph == phGame {
		if m.rr == nil {
			return m.gameView(vw, vh)
		}
		// dentro del juego los cuadros salen por el renderer de diferencias; bubbletea
		// se queda con un texto fijo para no redibujar la pantalla por su cuenta.
		if cv := m.gameCanvas(vw, vh); cv != nil {
			if time.Since(m.enteredAt) < 400*time.Millisecond {
				m.rr.full.Store(true)
			}
			m.rr.submit(cv.toFrame(m.rr.getFrame()))
		}
		return " "
	}
	if vw < 30 || vh < 12 {
		return fmt.Sprintf("%s\n\nTerminal muy pequeña (%dx%d).\nAgrándala a 30x12 o más.\nEn línea: %d", GameName, vw, vh, onlineCount())
	}
	t := time.Since(start).Seconds()
	cv := newCanvas(vw, vh)
	cv.mode = m.mode
	if m.ph == phChar {
		m.charView(cv, t)
		return cv.String()
	}
	hor := drawScene(cv, t)
	y := drawTitle(cv, t, hor)
	sub := "~ un MMORPG en ASCII bajo la luna ~"
	if len(sub) < vw {
		cv.text(-1, y, sub, RGB{150, 170, 220})
	}
	y += 2
	panelBg := RGB{6, 10, 30}
	switch m.ph {
	case phTitle:
		bw, bh := 26, len(menuItems)+4
		y = min(y, vh-bh-1)
		x0 := (vw - bw) / 2
		cv.box(x0, y, bw, bh, panelBg, RGB{90, 110, 190})
		for i, it := range menuItems {
			if i == m.sel {
				pulse := 0.5 + 0.5*math.Sin(t*5)
				label := "▸ " + it + " ◂"
				cv.text(-1, y+2+i, label, lerp(RGB{255, 200, 90}, RGB{255, 245, 200}, pulse))
			} else {
				cv.text(-1, y+2+i, it, RGB{160, 170, 200})
			}
		}
	case phHelp:
		lines := []string{
			"CÓMO JUGAR",
			"",
			"W A S D / flechas   moverte",
			"ESPACIO             hablar",
			"F / J               atacar",
			"E                   hablar con mercader",
			"I                   inventario",
			"L                   diario de misiones",
			"T / P               invitar / equipo",
			"/e mensaje          chat de equipo",
			"Z / X               poción vida / maná",
			"1 2 3               habilidades",
			"                    (nivel 1, 3, 5)",
			"Mata enemigos: XP, oro y niveles",
			"Entra al lago para hacer ondas",
			"Esc                 volver al menú",
			"q                   salir del juego",
			"",
			"Todos comparten el mismo mundo.",
			"[Enter] volver",
		}
		bw, bh := 40, len(lines)+2
		y = clamp(min(y, vh-bh-1), 0, vh)
		cv.box((vw-bw)/2, y, bw, bh, panelBg, RGB{90, 110, 190})
		for i, l := range lines {
			c := RGB{200, 210, 235}
			if i == 0 {
				c = RGB{255, 220, 120}
			}
			cv.text(-1, y+1+i, l, c)
		}
	case phLogin, phSignup:
		m.drawForm(cv, y, m.ph == phSignup)
	case phWelcome:
		m.drawWelcome(cv, y)
	case phName:
		bw, bh := 34, 7
		y = min(y, vh-bh-1)
		x0 := (vw - bw) / 2
		cv.box(x0, y, bw, bh, panelBg, RGB{90, 110, 190})
		cv.text(-1, y+1, "¿Cómo te llamas, viajero?", RGB{255, 220, 120})
		field := string(m.name)
		cur := " "
		if int(t*2)%2 == 0 {
			cur = "█"
		}
		cv.text(x0+5, y+3, "> "+field+cur, RGB{255, 255, 255})
		cv.text(-1, y+5, "[Enter] seguir  [Esc] volver", RGB{130, 140, 180})
	}
	foot := fmt.Sprintf(" ● %d en línea ", onlineCount())
	cv.text(1, vh-1, foot, RGB{120, 230, 140})
	hint := "↑↓/WS mover · Enter elegir "
	if len([]rune(hint))+len(foot)+3 < vw {
		cv.text(vw-len([]rune(hint))-1, vh-1, hint, RGB{110, 120, 160})
	}
	return cv.String()
}

func (m *model) gameView(vw, vh int) string {
	cv := m.gameCanvas(vw, vh)
	if cv == nil {
		return ""
	}
	return cv.String()
}

func (m *model) gameCanvas(vw, vh int) *canvas {
	size := byte('S')
	if vh < 26 || vw < 50 {
		size = 't'
	}
	vh--
	if vh < 1 {
		vh = 1
	}
	mu.Lock()
	if m.p == nil {
		mu.Unlock()
		return nil
	}
	px, py, n := m.p.X, m.p.Y, len(players)
	// el jugador va un poco abajo del centro (pies a 2/3 de la altura) para
	// que se vean los techos y las torres de arriba.
	ox, oy := clamp(px-vw/2, 0, max(0, W-vw)), clamp(py-vh*2/3, 0, max(0, H-vh))
	// zona muerta de cámara: la vista no se mueve mientras el jugador anda cerca
	// de su lugar. Al salirse, la cámara se desliza en unos cuantos cuadros
	// hasta recentrar y se vuelve a quedar quieta; con la cámara quieta solo
	// se reenvían los sprites que se mueven y no toda la pantalla.
	switch {
	case !m.camOK || m.camW != vw || m.camH != vh || abs(ox-m.camX) > vw || abs(oy-m.camY) > vh:
		m.camX, m.camY, m.camW, m.camH, m.camOK, m.camMove = ox, oy, vw, vh, true, false
	case m.camMove || abs(ox-m.camX) > vw/5 || abs(oy-m.camY) > vh/5:
		m.camX += camStep(ox - m.camX)
		m.camY += camStep(oy - m.camY)
		m.camMove = m.camX != ox || m.camY != oy
	}
	ox, oy = m.camX, m.camY
	f := composeInto(m.cbuf, time.Now(), size, ox, oy, vw, vh)
	m.cbuf = f
	type dot struct {
		x, y int
		me   bool
		c    RGB
		name string
	}
	var dots []dot
	if m.mini > 0 {
		for p := range players {
			c := p.Prim
			if c.R < 0 {
				c = defColor[p.Sprite%len(defColor)]
			}
			if samePartyP(p, m.p) && p != m.p {
				c = RGB{-2, 0, 0} // marcador de equipo
			}
			dots = append(dots, dot{p.X, p.Y, p == m.p, c, p.Name})
		}
	}
	name := m.p.Name
	st, sprite := m.p.St, m.p.Sprite
	st.Inv = append([]Item(nil), st.Inv...)
	nearNPC := npcNear(m.p)
	st.Quests = copyQuests(st.Quests)
	qlabels := questLabels(&m.p.St, ox, oy, vw, vh, size)
	qmarks := objectiveMarks(&m.p.St)
	safe := safeAt(px, py)
	fx := append([]Fx(nil), fxs...)
	type bub struct {
		x, y  int
		t     string
		party bool
	}
	var bubs []bub
	now := time.Now()
	rows := figRows(size)
	for p := range players {
		if p.Msg != "" && now.Sub(p.MsgAt) < bubbleLife && (!p.MsgParty || samePartyP(p, m.p)) {
			sx, sy := p.X-ox, p.Y-rows-1-oy
			if sx > -bubbleW && sx < vw+bubbleW && sy > -rows && sy < vh+rows {
				bubs = append(bubs, bub{sx, sy, p.Msg, p.MsgParty})
			}
		}
	}
	var logl []chatLine
	for _, l := range chatLog {
		if l.Party == nil || l.Party == m.p.Party {
			logl = append(logl, l)
		}
	}
	if len(logl) > 4 {
		logl = logl[len(logl)-4:]
	}
	prows := partyRows(m.p, ox, oy, vw, vh)
	var inviteFrom string
	if inv := m.p.Invite; inv != nil && time.Since(inv.At) < inviteTTL && players[inv.From] {
		inviteFrom = inv.From.Name
	}
	var nearOther string
	if o := nearestOther(m.p); o != nil {
		nearOther = o.Name
	}
	// nombres flotantes; se acomodan después con layoutTags
	var tags []tag
	var bars [][3]int // x, y, ancho de las barras de vida
	for p := range players {
		if p.X < ox-40 || p.X > ox+vw+40 || p.Y < oy || p.Y > oy+vh+rows {
			continue
		}
		t := tag{p.X - ox, p.Y - rows - oy, p.Name, RGB{230, 230, 230}, tagPlayer}
		switch {
		case p == m.p:
			t.c, t.pri = RGB{255, 255, 255}, tagSelf
		case samePartyP(p, m.p):
			t.c, t.pri = RGB{80, 240, 255}, tagParty
		}
		tags = append(tags, t)
	}
	for _, l := range qlabels {
		tags = append(tags, tag{l.x, l.y, l.text, npcTagColor(l.mark), tagNPC})
	}
	for _, e := range enemies {
		if (e.Dead && !now.Before(e.HitAt)) || e.X < ox-12 || e.X > ox+vw+12 || e.Y < oy-2 || e.Y > oy+vh+8 {
			continue
		}
		h := len(e.T.Art)
		c := RGB{230, 230, 230}
		if e.Target != nil {
			c = RGB{255, 120, 100}
		}
		tags = append(tags, tag{e.X - ox, e.Y - h - 1 - oy, fmt.Sprintf("%s Nv%d", e.T.Name, e.Lvl), c, tagEnemy})
		bars = append(bars, [3]int{e.X - ox - 4, e.Y - h - oy, 8})
	}
	mu.Unlock()

	cv := m.cvbuf
	if cv == nil || cv.w != vw || len(cv.c) < vh || cap(cv.c) < vh+1 {
		cv = newCanvas(vw, vh)
		cv.c = append(make([][]TCell, 0, vh+1), cv.c...)
		m.cvbuf = cv
	}
	cv.c, cv.h = cv.c[:vh], vh
	cv.mode = m.mode
	for y := 0; y < vh; y++ {
		for x := 0; x < vw; x++ {
			cv.c[y][x] = TCell{f[y][x].Ch, f[y][x].C, RGB{-1, 0, 0}}
		}
	}
	if m.mini > 0 {
		mw, mh := min(64, vw/2), min(24, vh/2)
		x0, y0 := vw-mw-2, 1
		if m.mini == 2 { // mapa a pantalla completa con nombres
			mw, mh, x0, y0 = vw-2, vh-2, 1, 1
		}
		if mw >= 12 && mh >= 6 {
			mm := minimap(mw, mh)
			cv.box(x0-1, y0-1, mw+2, mh+2, RGB{6, 10, 30}, RGB{150, 160, 200})
			for y := 0; y < mh; y++ {
				for x := 0; x < mw; x++ {
					cv.c[y0+y][x0+x] = TCell{' ', RGB{0, 0, 0}, mm[y][x]}
				}
			}
			rx0, ry0 := ox*mw/W, oy*mh/H
			rx1, ry1 := min(mw-1, (ox+vw)*mw/W), min(mh-1, (oy+vh)*mh/H)
			for x := rx0; x <= rx1; x++ {
				cv.c[y0+ry0][x0+x].Ch, cv.c[y0+ry0][x0+x].Fg = '·', RGB{255, 255, 255}
				cv.c[y0+ry1][x0+x].Ch, cv.c[y0+ry1][x0+x].Fg = '·', RGB{255, 255, 255}
			}
			// marcadores de lugares (con nombre en el mapa grande)
			used := make([][]bool, mh)
			for i := range used {
				used[i] = make([]bool, mw)
			}
			mark := make([][]bool, mh)
			for i := range mark {
				mark[i] = make([]bool, mw)
			}
			for _, l := range locs {
				x, y := l.X*mw/W, l.Y*mh/H
				// dos lugares en la misma celda: el segundo se corre a un lado
				for _, d := range [...]int{0, 1, -1, 2, -2} {
					if xx := x + d; xx >= 0 && xx < mw && !mark[y][xx] {
						x = xx
						break
					}
				}
				mark[y][x], used[y][x] = true, true
				cv.c[y0+y][x0+x].Ch, cv.c[y0+y][x0+x].Fg = '◆', RGB{255, 230, 120}
				if m.mini == 2 {
					nm := []rune(l.Name)
					lx := x + 2
					if lx+len(nm) >= mw {
						lx = x - 1 - len(nm)
					}
					ok := lx >= 0 && lx+len(nm) < mw
					for i := -1; ok && i <= len(nm); i++ {
						if xx := lx + i; xx >= 0 && xx < mw && used[y][xx] {
							ok = false
						}
					}
					if ok {
						for i, ch := range nm {
							cv.c[y0+y][x0+lx+i].Ch, cv.c[y0+y][x0+lx+i].Fg = ch, RGB{255, 255, 255}
							cv.c[y0+y][x0+lx+i].Bg = lerp(cv.c[y0+y][x0+lx+i].Bg, RGB{0, 0, 0}, 0.6)
							used[y][lx+i] = true
						}
					}
				}
			}
			blink := time.Now().UnixMilli()/400%2 == 0
			for _, q := range qmarks {
				x, y := q[0]*mw/W, q[1]*mh/H
				if x >= 0 && x < mw && y >= 0 && y < mh {
					cv.c[y0+y][x0+x].Ch, cv.c[y0+y][x0+x].Fg = '★', RGB{255, 240, 80}
				}
			}
			sort.SliceStable(dots, func(i, j int) bool { return !dots[i].me && dots[j].me })
			// varios jugadores en la misma celda se muestran con un número
			count := map[[2]int]int{}
			for _, d := range dots {
				count[[2]int{d.x * mw / W, d.y * mh / H}]++
			}
			for _, d := range dots {
				x, y := d.x*mw/W, d.y*mh/H
				ch, c := '●', d.c
				if c.R == -2 {
					ch, c = '◉', RGB{80, 240, 255}
					nm := []rune(d.name)
					if len(nm) > 10 {
						nm = nm[:10]
					}
					ok := x+2+len(nm) < mw
					for i := -1; ok && i <= len(nm); i++ {
						if used[y][x+2+i] {
							ok = false
						}
					}
					for i, r := range nm {
						if ok {
							cv.c[y0+y][x0+x+2+i].Ch, cv.c[y0+y][x0+x+2+i].Fg = r, RGB{80, 240, 255}
							used[y][x+2+i] = true
						}
					}
				}
				if n := count[[2]int{x, y}]; n > 1 && !d.me {
					ch = rune('0' + min(n, 9))
				}
				if d.me {
					ch = '@'
					if blink {
						c = RGB{255, 255, 255}
					}
				}
				cv.c[y0+y][x0+x].Ch, cv.c[y0+y][x0+x].Fg = ch, c
			}
			title := " Mapa · M: cerrar "
			if m.mini == 1 {
				title = " Mapa · M: ampliar "
			}
			cv.text(x0+1, y0-1, title, RGB{255, 220, 120})
		}
	}
	now2 := time.Now()
	for _, f := range fx {
		el := now2.Sub(f.Start)
		if el < 0 {
			continue
		}
		if f.Proj {
			drawProj(cv, f, el, ox, oy, vw, vh)
			continue
		}
		y := f.Y - oy
		if f.Rise {
			y -= int(el.Seconds() * 4)
		}
		r := []rune(f.Text)
		x := f.X - ox - len(r)/2
		for i, ch := range r {
			if xx := x + i; xx >= 0 && xx < vw && y >= 0 && y < vh {
				cv.c[y][xx].Ch, cv.c[y][xx].Fg = ch, f.C
			}
		}
	}
	if m.mini != 2 {
		busy := make([][]bool, vh)
		for y := range busy {
			busy[y] = make([]bool, vw)
		}
		for _, b := range bars {
			for x := b[0]; x < b[0]+b[2]; x++ {
				if x >= 0 && x < vw && b[1] >= 0 && b[1] < vh {
					busy[b[1]][x] = true
				}
			}
		}
		if m.mini == 1 { // el minimapa no se tapa
			mw, mh := min(64, vw/2), min(24, vh/2)
			for y := 0; y < mh+2 && y < vh; y++ {
				for x := max(0, vw-mw-3); x < vw; x++ {
					busy[y][x] = true
				}
			}
		}
		layoutTags(cv, tags, busy)
	}
	if m.mini != 2 && m.win != 6 {
		drawHUD(cv, st, sprite, now2)
	}
	if safe != nil {
		t := " ☾ Zona segura · " + safe.Name + " "
		cv.text(max(0, vw-len([]rune(t))-1), 0, t, RGB{140, 255, 170})
	}
	if m.mini == 0 {
		drawPartyPanel(cv, prows, 1)
	}
	if inviteFrom != "" && m.win == 0 {
		t := inviteFrom + " te invita a su equipo   [S]í / [N]o"
		w := len([]rune(t)) + 6
		y := vh / 4
		cv.box((vw-w)/2, y-1, w, 3, RGB{10, 30, 40}, RGB{80, 240, 255})
		cv.text(-1, y, t, RGB{220, 255, 255})
	} else if nearOther != "" && nearNPC == nil && m.win == 0 {
		cv.text(-1, vh-2, " Pulsa T para invitar a "+nearOther+" al equipo ", RGB{120, 230, 255})
	}
	if nearNPC != nil && m.win == 0 {
		verb := "comerciar con "
		if nearNPC.Kind == 'q' {
			verb = "hablar con "
		}
		cv.text(-1, vh-2, " Pulsa E para "+verb+nearNPC.Name+" ", RGB{255, 220, 120})
	}
	m.drawWin(cv, st, sprite)
	if m.win != 0 && safe == nil {
		// con un menú abierto el mundo sigue: fuera de zona segura te avisamos
		t := " ⚠ Estás en peligro "
		x := (vw - len([]rune(t))) / 2
		for i, ch := range []rune(t) {
			if xx := x + i; xx >= 0 && xx < vw {
				cv.c[0][xx] = TCell{ch, RGB{255, 240, 240}, RGB{170, 20, 20}}
			}
		}
	}
	mu.Lock()
	ann := curAnnounce
	mu.Unlock()
	drawAnnouncement(cv, ann, now2)
	for _, b := range bubs {
		cv.drawBubble(b.x, b.y, b.t, b.party)
	}
	// últimos mensajes del chat, abajo a la izquierda
	for i, l := range logl {
		y := vh - len(logl) - 1 + i
		if m.typing {
			y--
		}
		line := []rune(" " + l.Name + ": " + l.Text + " ")
		if len(line) > vw/2 {
			line = append(line[:max(0, vw/2-1)], '…')
		}
		for j, ch := range line {
			if y >= 0 && j < vw {
				c := RGB{220, 220, 240}
				if l.Party != nil {
					c = RGB{130, 240, 255}
				}
				if j <= len([]rune(l.Name)) {
					c = RGB{255, 210, 110}
					if l.Party != nil {
						c = RGB{80, 220, 240}
					}
				}
				cv.c[y][j] = TCell{ch, c, RGB{6, 10, 30}}
			}
		}
	}
	if m.typing && vh > 1 {
		cur := " "
		if time.Now().UnixMilli()/400%2 == 0 {
			cur = "█"
		}
		in := []rune(" Decir: " + string(m.input) + cur)
		if len(in) > vw {
			in = in[len(in)-vw:]
		}
		for x := 0; x < vw; x++ {
			ch := ' '
			if x < len(in) {
				ch = in[x]
			}
			cv.c[vh-1][x] = TCell{ch, RGB{255, 255, 255}, RGB{30, 40, 90}}
		}
	}
	zone := zoneAt(px, py)
	status := []rune(fmt.Sprintf(" ◆ %s | %s @ %d,%d | en línea: %d | WASD mover · F atacar · 1-3 hab. · E tienda · I inventario · L misiones · T invitar · P equipo · Z/X pociones · ESPACIO hablar · M mapa · Esc menú · q salir", zone, name, px, py, n))
	if len(status) > vw {
		status = status[:max(0, vw)]
	}
	srow := make([]TCell, vw)
	for x := range srow {
		ch := ' '
		if x < len(status) {
			ch = status[x]
		}
		srow[x] = TCell{ch, RGB{10, 10, 20}, RGB{200, 200, 210}}
	}
	cv.c = append(cv.c, srow)
	cv.h++
	return cv
}

// camStep avanza un tercio de lo que falta, al menos una celda.
func camStep(d int) int {
	switch {
	case d > 0:
		return max(1, (d+2)/3)
	case d < 0:
		return min(-1, (d-2)/3)
	}
	return 0
}

func cl(v float64) int { return clamp(int(v), 0, 255) }
func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// programHandler arma el programa de bubbletea de cada sesión y limpia al salir.
func programHandler(s ssh.Session) *tea.Program {
	tm, opts := handler(s)
	m := tm.(*model)
	if ipBanned(m.remote) {
		log.Info("banned ip", "ip", m.remote)
		s.Write([]byte("\r\nLAGO LUNAR: tu IP está baneada de este servidor.\r\n"))
		s.Exit(1)
		return nil
	}
	pty, _, _ := s.Pty()
	register(m, s, pty.Term)
	// net.TCPConn ya trae TCP_NODELAY activado.
	p := tea.NewProgram(m, append(opts, tea.WithInput(s), tea.WithOutput(m.out), tea.WithFPS(30))...)
	go func() {
		<-s.Context().Done()
		p.Wait() // limpiar hasta que termine el loop de eventos, sin carrera con Update
		m.rr.stop()
		m.leave()
		m.logout()
		unregister(m)
	}()
	return p
}

func handler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()
	name := []rune(s.User())
	if len(name) > 12 {
		name = name[:12]
	}
	mode := detectMode(pty.Term, s.Environ())
	log.Info("session", "user", s.User(), "term", pty.Term, "mode", mode.String(), "w", pty.Window.Width, "h", pty.Window.Height, "remote", s.RemoteAddr().String())
	m := model{user: string(name), name: name, char: rand.Intn(len(sprites)), prim: 0, sec: 0, w: pty.Window.Width, h: pty.Window.Height, mode: mode}
	mp := &m
	if h, _, err := net.SplitHostPort(s.RemoteAddr().String()); err == nil {
		m.remote = h
	}
	mp.out = &lockedW{w: s}
	mp.rr = newRawRenderer(mp.out, mode)

	return mp, []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		os.Exit(adminCLI(os.Args[2:]))
	}
	if len(os.Args) > 2 && os.Args[1] == "worldmap" {
		genMap()
		if err := writeWorldPNG(os.Args[2]); err != nil {
			log.Fatal(err)
		}
		return
	}
	if len(os.Args) > 3 && os.Args[1] == "snapshot" { // render sin servidor: snapshot x,y personaje [mini]
		genMap()
		var x, y, ch int
		fmt.Sscanf(os.Args[2], "%d,%d", &x, &y)
		fmt.Sscanf(os.Args[3], "%d", &ch)
		m := &model{w: 100, h: 30, mode: cm256, ph: phGame, user: "Xoco", name: []rune("Xoco"), char: ch}
		os.Setenv("SPAWN", os.Args[2])
		m.join()
		if len(os.Args) > 4 {
			fmt.Sscanf(os.Args[4], "%d", &m.mini)
		}
		_ = x
		_ = y
		os.Stdout.WriteString("\x1b[2J\x1b[H" + strings.ReplaceAll(m.View(), "\n", "\r\n"))
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "gallery" {
		gallery()
		return
	}
	genMap()
	initSafeZones()
	spawnEnemies()
	placeNPCs()
	for _, n := range npcs {
		log.Info("npc", "name", n.Name, "x", n.X, "y", n.Y)
	}
	go simLoop()
	if err := loadAccounts(); err != nil {
		log.Fatal("cuentas", "err", err)
	}
	log.Info("accounts", "path", accPath, "n", len(accounts))
	loadBans()
	startBackups()
	go onlineWriter()
	port := "2222"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort("0.0.0.0", port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithPasswordAuth(func(ssh.Context, string) bool { return true }),
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithMiddleware(bm.MiddlewareWithProgramHandler(programHandler, termenv.Ascii)),
	)
	if err != nil {
		log.Fatal(err)
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	log.Info("listening", "port", port)
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-done
	ctx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	s.Shutdown(ctx)
}

const (
	stepEvery  = 50 * time.Millisecond  // máximo un paso por intervalo
	holdWindow = 100 * time.Millisecond // la tecla cuenta como sostenida si llegó una repetición en este lapso
)

// stepMove corre en cada tick: avanza a lo mucho un paso en la última dirección
// y se detiene cuando dejan de llegar repeticiones (los terminales no mandan key-up).
func (m *model) stepMove(now time.Time) {
	if m.ph != phGame || m.p == nil || m.typing || (m.dx == 0 && m.dy == 0) {
		return
	}
	if now.Sub(m.dirAt) > holdWindow {
		m.dx, m.dy = 0, 0
		return
	}
	if m.held && now.Sub(m.stepAt) >= stepEvery {
		m.step(now)
	}
}

func (m *model) step(now time.Time) {
	m.stepAt = now
	dx, dy := m.dx, m.dy
	mu.Lock()
	defer mu.Unlock()
	if m.p == nil {
		return
	}
	nx, ny := m.p.X+2*dx, m.p.Y+dy
	if dx != 0 {
		m.p.Left = dx < 0
	}
	if !blocked(nx, ny) && !npcBlocked(nx, ny) {
		m.p.X, m.p.Y = nx, ny
		m.p.Steps++
		m.p.LastMove = now
		questMove(m.p)
		if isWater(nx, ny) {
			ripples = append(ripples, Ripple{nx, ny, now})
		}
	}
}
