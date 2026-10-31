// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// enemigos
type EnemyType struct {
	Name   string
	Art    []string
	C      RGB
	HP     int
	Atk    int
	XP     int
	Gold   int
	StepMs int
}

var enemyTypes = []EnemyType{
	{"Slime", []string{" .--. ", "( oo )", "`----'"}, RGB{120, 230, 120}, 30, 4, 8, 2, 520},
	{"Lobo", []string{"   /\\_/\\ ", " _( o.o)>", "/__|_|_| "}, RGB{180, 180, 200}, 45, 7, 14, 4, 230},
	{"Esqueleto", []string{" .-. ", "(o.o)", " |=| ", "/| |\\", " / \\ "}, RGB{235, 230, 210}, 70, 11, 24, 7, 360},
	{"Orco", []string{" ,---. ", "( O O )", " \\_^_/ ", "/|###|\\", " /   \\ "}, RGB{110, 170, 70}, 120, 16, 40, 12, 420},
	{"Dragón joven", []string{"   __/\\__  ", " <(  @ @)>~", "/\\\\/ \\/\\/\\", " /_/  \\_\\  "}, RGB{230, 90, 60}, 220, 26, 80, 25, 300},
}

// Enemy es un monstruo del mapa. Reaparece en su punto de origen al morir.
type Enemy struct {
	T            *EnemyType
	Lvl          int
	X, Y         int
	HomeX, HomeY int
	HP, MaxHP    int
	Atk          int
	Dead         bool
	RespawnAt    time.Time
	NextStep     time.Time
	NextAtk      time.Time
	HitAt        time.Time
	KnockDir     int
	Target       *Player
	Left         bool
}

var enemies []*Enemy // protegido por mu

const (
	maxEnemies = 900
	aggroX     = 40
	aggroY     = 14
	leash      = 90
	activeX    = 160
	activeY    = 70
)

func distLake(x, y int) float64 {
	return math.Hypot(float64(x-lakeCX)/2, float64(y-lakeCY))
}

func tierAt(x, y int) (types []int, lvl int) {
	d := distLake(x, y)
	lvl = 1 + int(d/70)
	switch {
	case d < 190:
		return []int{0, 1}, lvl
	case d < 360:
		return []int{1, 2}, lvl
	case d < 600:
		return []int{2, 3}, lvl
	default:
		return []int{3, 4}, lvl
	}
}

func newEnemy(x, y int) *Enemy {
	ts, lvl := tierAt(x, y)
	t := &enemyTypes[ts[rand.Intn(len(ts))]]
	f := 1 + 0.18*float64(lvl-1)
	hp := int(float64(t.HP) * f)
	return &Enemy{T: t, Lvl: lvl, X: x, Y: y, HomeX: x, HomeY: y, HP: hp, MaxHP: hp, Atk: int(float64(t.Atk) * f)}
}

func spawnEnemies() {
	enemies = nil
	r := rand.New(rand.NewSource(7))
	for tries := 0; len(enemies) < 140 && tries < 50000; tries++ { // más enemigos fáciles alrededor del Lago Lunar
		ang := r.Float64() * 2 * math.Pi
		d := 95 + r.Float64()*85
		x, y := lakeCX+int(math.Cos(ang)*d*2), lakeCY+int(math.Sin(ang)*d)
		if blocked(x, y) || isWater(x, y) || safeAt(x, y) != nil {
			continue
		}
		enemies = append(enemies, newEnemy(x, y))
	}
	for tries := 0; len(enemies) < maxEnemies && tries < 200000; tries++ {
		x, y := 20+r.Intn(W-40), 20+r.Intn(H-30)
		if blocked(x, y) || isWater(x, y) || distLake(x, y) < 70 || safeAt(x, y) != nil {
			continue
		}
		enemies = append(enemies, newEnemy(x, y))
	}
}

// estadísticas del jugador
type Class struct {
	Name     string
	Specials [3]Special
}

// Special es una habilidad de clase (teclas 1-3).
type Special struct {
	Name  string
	Kind  byte // m cuerpo a cuerpo, l línea, a área, h curación
	Mult  float64
	Range int
	MP    int
	CD    time.Duration
	Heal  float64
	Lvl   int
	C     RGB
	Glyph rune
}

var classes = []Class{
	{"Guerrero", [3]Special{
		{"Tajo Giratorio", 'a', 1.5, 14, 8, 3 * time.Second, 0, 1, RGB{255, 230, 150}, '✶'},
		{"Golpe Brutal", 'm', 3.2, 16, 12, 5 * time.Second, 0, 3, RGB{255, 140, 80}, '✸'},
		{"Grito de Guerra", 'a', 2.2, 22, 20, 12 * time.Second, 0.2, 5, RGB{255, 90, 60}, '!'},
	}},
	{"Mago", [3]Special{
		{"Bola de Fuego", 'l', 2.0, 34, 8, 2 * time.Second, 0, 1, RGB{255, 140, 40}, '●'},
		{"Nova de Hielo", 'a', 1.8, 18, 14, 6 * time.Second, 0, 3, RGB{140, 220, 255}, '❄'},
		{"Meteoro", 'a', 3.5, 26, 25, 12 * time.Second, 0, 5, RGB{255, 80, 40}, '✹'},
	}},
	{"Arquero", [3]Special{
		{"Flecha Certera", 'l', 1.9, 44, 6, 2 * time.Second, 0, 1, RGB{220, 255, 160}, '→'},
		{"Lluvia de Flechas", 'a', 1.6, 20, 14, 6 * time.Second, 0, 3, RGB{200, 240, 140}, '↓'},
		{"Disparo Perforante", 'l', 3.0, 60, 22, 10 * time.Second, 0, 5, RGB{255, 255, 200}, '═'},
	}},
	{"Paladín", [3]Special{
		{"Golpe Sagrado", 'm', 2.2, 16, 8, 3 * time.Second, 0, 1, RGB{255, 240, 140}, '✚'},
		{"Curación", 'h', 0, 0, 14, 8 * time.Second, 0.4, 3, RGB{140, 255, 160}, '+'},
		{"Juicio Divino", 'a', 3.0, 22, 24, 12 * time.Second, 0, 5, RGB{255, 255, 200}, '✦'},
	}},
	{"Oscuro", [3]Special{
		{"Drenar Vida", 'm', 1.8, 18, 8, 3 * time.Second, 0.5, 1, RGB{200, 80, 220}, '♥'},
		{"Llamas Oscuras", 'a', 1.8, 18, 14, 6 * time.Second, 0, 3, RGB{170, 60, 255}, '♨'},
		{"Pacto Demoníaco", 'a', 3.5, 24, 26, 12 * time.Second, 0, 5, RGB{255, 40, 90}, '✠'},
	}},
}

// índice de sprite -> clase
var spriteClass = []int{0, 3, 0, 1, 4, 0, 2, 0, 1, 4}

func classOf(sprite int) *Class { return &classes[spriteClass[sprite%len(spriteClass)]] }

// Stats guarda nivel, vida, maná, oro, inventario y tiempos de combate del jugador.
type Stats struct {
	Lvl, XP     int
	HP, MP      int
	Gold        int
	NextAtk     time.Time
	CD          [3]time.Time
	LevelUpAt   time.Time
	DiedAt      time.Time
	HurtAt      time.Time
	SlashAt     time.Time
	AtkSk       int // -1 ataque normal, 0..2 habilidad
	Msg         string
	MsgAt       time.Time
	lastRegen   time.Time
	lastMPRegen time.Time
	Inv         []Item
	Weapon      string
	Armor       string
	Quests      map[string]QState
}

func xpNeed(l int) int   { return 20*l + 10*l*l }
func maxHP(l int) int    { return 80 + 22*(l-1) }
func maxMP(l int) int    { return 30 + 8*(l-1) }
func attackOf(l int) int { return 9 + 3*(l-1) }

// MaxHP incluye el bono de armadura.
func (p *Player) MaxHP() int { return p.St.maxHP() }

// MaxMP es el maná máximo según el nivel.
func (p *Player) MaxMP() int { return p.St.maxMP() }

// efectos

// Fx es un efecto visual temporal: texto, número de daño o proyectil.
type Fx struct {
	X, Y  int
	Text  string
	C     RGB
	Start time.Time
	Dur   time.Duration
	Rise  bool
	// proyectil: va de (X,Y) a (X1,Y1) en Dur dejando una estela
	Proj   bool
	X1, Y1 int
	Trail  rune
	TC     RGB
}

var fxs []Fx // protegido por mu

func addFx(x, y int, text string, c RGB, dur time.Duration, rise bool) {
	if len(fxs) > 400 {
		fxs = fxs[1:]
	}
	fxs = append(fxs, Fx{X: x, Y: y, Text: text, C: c, Start: time.Now().Add(fxDelay), Dur: dur, Rise: rise})
}

func burst(x, y, r int, g rune, c RGB) {
	for a := 0; a < 12; a++ {
		ang := float64(a) / 12 * 2 * math.Pi
		addFx(x+int(math.Cos(ang)*float64(r)), y+int(math.Sin(ang)*float64(r)/2.2), string(g), c, 450*time.Millisecond, false)
	}
}

// combate (con mu tomado)
func reach() int { return figCols('S')/2 + 8 }

func (p *Player) facing(e *Enemy, rng int) bool {
	dx, dy := e.X-p.X, e.Y-p.Y
	if dy < -3 || dy > 3 {
		return false
	}
	if p.Left {
		return dx <= 2 && dx >= -rng
	}
	return dx >= -2 && dx <= rng
}

func (p *Player) damage(mult float64) int {
	b := float64(p.St.atk()) * mult
	return max(1, int(b*(0.85+0.3*rand.Float64())))
}

func nearestFacing(p *Player, rng int) *Enemy {
	var best *Enemy
	bd := 1 << 30
	for _, e := range enemies {
		if e.Dead || !p.facing(e, rng) {
			continue
		}
		if d := abs(e.X-p.X) + 2*abs(e.Y-p.Y); d < bd {
			best, bd = e, d
		}
	}
	return best
}

func hitEnemy(p *Player, e *Enemy, dmg int, c RGB) {
	e.HP -= dmg
	e.HitAt = time.Now().Add(fxDelay)
	e.KnockDir = 1
	if e.X < p.X {
		e.KnockDir = -1
	}
	e.Target = p
	addFx(e.X, e.Y-len(e.T.Art)-1, fmt.Sprint(dmg), c, 900*time.Millisecond, true)
	if e.HP <= 0 {
		e.Dead = true
		e.RespawnAt = time.Now().Add(time.Duration(18+rand.Intn(12)) * time.Second)
		xp := e.T.XP * (1 + (e.Lvl-1)/2)
		gold := e.T.Gold + rand.Intn(e.T.Gold+1)
		p.St.Gold += gold
		sh := sharers(p, e.X, e.Y)
		each := splitXP(xp, len(sh))
		label := fmt.Sprintf("+%d XP  +%d oro", each, gold)
		if len(sh) > 1 {
			label += fmt.Sprintf(" (equipo x%d)", len(sh))
		}
		addFx(e.X, e.Y-1, label, RGB{255, 220, 90}, 1400*time.Millisecond, true)
		for _, mbr := range sh {
			mbr.gainXP(each)
			questKill(mbr, e)
		}
		maybeDrop(p, e)
	}
}

func (p *Player) gainXP(xp int) {
	p.St.XP += xp
	for p.St.XP >= xpNeed(p.St.Lvl) {
		p.St.XP -= xpNeed(p.St.Lvl)
		p.St.Lvl++
		p.St.HP, p.St.MP = p.MaxHP(), p.MaxMP()
		p.St.LevelUpAt = time.Now()
		addFx(p.X, p.Y-12, fmt.Sprintf("¡NIVEL %d!", p.St.Lvl), RGB{255, 240, 120}, 2*time.Second, true)
	}
}

// basicAttack es el ataque normal (F/J).
func basicAttack(p *Player) {
	now := time.Now()
	if now.Before(p.St.NextAtk) {
		return
	}
	p.St.NextAtk = now.Add(380 * time.Millisecond)
	p.St.SlashAt = now
	e := nearestFacing(p, reach())
	if e == nil { // voltear solo si hay un enemigo justo atrás
		p.Left = !p.Left
		if e = nearestFacing(p, reach()); e == nil {
			p.Left = !p.Left
		}
	}
	dir := 1
	if p.Left {
		dir = -1
	}
	startAttack(p, -1)
	if e != nil {
		basicFx(p, e, dir)
		withDelay(impactDelay(p, e), func() { hitEnemy(p, e, p.damage(1), RGB{255, 255, 255}) })
	} else {
		basicFx(p, nil, dir)
	}
}

// special lanza la habilidad 1-3 de la clase.
func special(p *Player, i int) {
	cl := classOf(p.Sprite)
	s := cl.Specials[i]
	now := time.Now()
	switch {
	case p.St.Lvl < s.Lvl:
		p.say2(fmt.Sprintf("%s se desbloquea en nivel %d", s.Name, s.Lvl))
		return
	case now.Before(p.St.CD[i]):
		p.say2(fmt.Sprintf("%s en espera: %.1fs", s.Name, p.St.CD[i].Sub(now).Seconds()))
		return
	case p.St.MP < s.MP:
		p.say2("¡Sin MP suficiente!")
		return
	}
	p.St.MP -= s.MP
	p.St.CD[i] = now.Add(s.CD)
	p.say2(s.Name + "!")
	dir := 1
	if p.Left {
		dir = -1
	}
	startAttack(p, i)
	specialFx(p, s, dir)
}

func (p *Player) heal(n int) {
	if n <= 0 {
		return
	}
	p.St.HP = min(p.MaxHP(), p.St.HP+n)
	addFx(p.X, p.Y-10, fmt.Sprintf("+%d", n), RGB{120, 255, 140}, 900*time.Millisecond, true)
}

func (p *Player) say2(s string) { p.St.Msg, p.St.MsgAt = s, time.Now() }

func hurtPlayer(p *Player, dmg int) {
	if safeAt(p.X, p.Y) != nil {
		return
	}
	dmg = max(1, dmg-p.St.def()/2)
	p.St.HP -= dmg
	p.St.HurtAt = time.Now()
	addFx(p.X, p.Y-8, fmt.Sprint(-dmg), RGB{255, 80, 80}, 800*time.Millisecond, true)
	if p.St.HP <= 0 {
		// al morir: reaparece en Lago Lunar y pierde 10% de XP del nivel y 10% del oro
		p.St.XP -= p.St.XP / 10
		p.St.Gold -= p.St.Gold / 10
		p.St.HP, p.St.MP = p.MaxHP(), p.MaxMP()/2
		p.St.DiedAt = time.Now()
		p.X, p.Y = spawnPoint()
		for _, e := range enemies {
			if e.Target == p {
				e.Target = nil
			}
		}
	}
}

// simulación
func simLoop() {
	t := time.NewTicker(100 * time.Millisecond)
	for now := range t.C {
		mu.Lock()
		simStep(now)
		mu.Unlock()
	}
}

// simStep avanza la simulación: regeneración, IA de enemigos y efectos.
func simStep(now time.Time) {
	// regeneración
	for p := range players {
		if p.St.Lvl == 0 {
			continue
		}
		if now.Sub(p.St.lastRegen) > 2*time.Second {
			p.St.lastRegen = now
			if now.Sub(p.St.HurtAt) > 4*time.Second {
				p.St.HP = min(p.MaxHP(), p.St.HP+1+p.MaxHP()/40)
			}
		}
		if now.Sub(p.St.lastMPRegen) > time.Second {
			p.St.lastMPRegen = now
			p.St.MP = min(p.MaxMP(), p.St.MP+1)
		}
	}
	for p := range players {
		if p.Invite != nil && now.Sub(p.Invite.At) > inviteTTL {
			p.Invite = nil
		}
	}
	cleanupSolo()
	// quitar efectos vencidos
	alive := fxs[:0]
	for _, f := range fxs {
		if now.Sub(f.Start) < f.Dur {
			alive = append(alive, f)
		}
	}
	fxs = alive
	if len(players) == 0 {
		return
	}
	for _, e := range enemies {
		near := false
		for p := range players {
			if abs(p.X-e.X) < activeX && abs(p.Y-e.Y) < activeY {
				near = true
				break
			}
		}
		if !near {
			continue
		}
		if e.Dead {
			if now.After(e.RespawnAt) {
				ne := newEnemy(e.HomeX, e.HomeY)
				*e = *ne
			}
			continue
		}
		if e.Target != nil && !players[e.Target] {
			e.Target = nil
		}
		if e.Target == nil {
			for p := range players {
				if p.St.Lvl > 0 && abs(p.X-e.X) <= aggroX && abs(p.Y-e.Y) <= aggroY && now.Sub(p.St.DiedAt) > 3*time.Second && !isWater(p.X, p.Y) && safeAt(p.X, p.Y) == nil {
					e.Target = p
					break
				}
			}
		}
		if e.Target != nil && (abs(e.X-e.HomeX) > leash || abs(e.Y-e.HomeY) > leash/2 || safeAt(e.Target.X, e.Target.Y) != nil) {
			e.Target = nil
		}
		if now.Before(e.NextStep) {
			continue
		}
		e.NextStep = now.Add(time.Duration(e.T.StepMs) * time.Millisecond)
		tx, ty := e.HomeX+rand.Intn(21)-10, e.HomeY+rand.Intn(9)-4
		if p := e.Target; p != nil {
			dx, dy := p.X-e.X, p.Y-e.Y
			reachX := figCols('S')/2 + 4
			if abs(dx) <= reachX && abs(dy) <= 2 {
				if now.After(e.NextAtk) {
					e.NextAtk = now.Add(1200 * time.Millisecond)
					hurtPlayer(p, max(1, int(float64(e.Atk)*(0.8+0.4*rand.Float64()))))
				}
				continue
			}
			tx, ty = p.X-sign(dx)*(reachX-2), p.Y
		} else if rand.Intn(3) != 0 {
			continue
		}
		mx, my := 0, 0
		if abs(tx-e.X) > 1 {
			mx = 2 * sign(tx-e.X)
		}
		if ty != e.Y {
			my = sign(ty - e.Y)
		}
		if mx != 0 && (my == 0 || rand.Intn(2) == 0) {
			my = 0
		} else {
			mx = 0
		}
		nx, ny := e.X+mx, e.Y+my
		if (mx != 0 || my != 0) && !blocked(nx, ny) && !isWater(nx, ny) && !crowded(e, nx, ny) && safeAt(nx, ny) == nil {
			if mx != 0 {
				e.Left = mx < 0
			}
			e.X, e.Y = nx, ny
		}
	}
}

func sign(a int) int {
	switch {
	case a > 0:
		return 1
	case a < 0:
		return -1
	}
	return 0
}

// drawEnemies dibuja en la capa de composición (con mu tomado).
func drawEnemies(layer map[int]Cell, vx, vy, vw, vh int, now time.Time) {
	for _, e := range enemies {
		if (e.Dead && !now.Before(e.HitAt)) || e.X < vx-12 || e.X > vx+vw+12 || e.Y < vy-2 || e.Y > vy+vh+8 {
			continue
		}
		h := len(e.T.Art)
		c := e.T.C
		kx := 0
		if el := now.Sub(e.HitAt); el >= 0 && el < 150*time.Millisecond {
			c = RGB{255, 255, 255}
			kx = e.KnockDir
		} else if el >= 0 && el < 260*time.Millisecond {
			c = lerp(e.T.C, RGB{255, 120, 120}, 0.5)
		}
		ex := e.X + kx
		for r, line := range e.T.Art {
			lr := []rune(line)
			for i, ch := range lr {
				if ch == ' ' {
					continue
				}
				if e.Left {
					ch = mirror(ch)
					i = len(lr) - 1 - i
				}
				layer[(e.Y-h+1+r)*W+ex-len(lr)/2+i] = Cell{ch, c}
			}
		}
		// barra de vida (el nombre lo pone layoutTags para que no se encime)
		bw := 8
		fill := int(math.Ceil(float64(bw) * float64(e.HP) / float64(e.MaxHP)))
		for i := 0; i < bw; i++ {
			ch, cc := '▬', RGB{80, 30, 30}
			if i < fill {
				cc = RGB{220, 60, 60}
			}
			layer[(e.Y-h)*W+e.X-bw/2+i] = Cell{ch, cc}
		}
	}
}

func mirror(r rune) rune {
	switch r {
	case '/':
		return '\\'
	case '\\':
		return '/'
	case '(':
		return ')'
	case ')':
		return '('
	case '<':
		return '>'
	case '>':
		return '<'
	}
	return r
}

func bar(n, max, w int) string {
	if max <= 0 {
		max = 1
	}
	f := clamp(n*w/max, 0, w)
	return strings.Repeat("█", f) + strings.Repeat("░", w-f)
}

func drawHUD(cv *canvas, st Stats, sprite int, now time.Time) {
	if cv.w < 30 || cv.h < 8 {
		return
	}
	cl := classOf(sprite)
	bw := 30
	lines := []struct {
		t string
		c RGB
	}{
		{fmt.Sprintf("Nivel %d · %s", st.Lvl, cl.Name), RGB{255, 220, 120}},
		{"HP " + bar(st.HP, st.maxHP(), 14) + fmt.Sprintf(" %d/%d", st.HP, st.maxHP()), RGB{240, 90, 90}},
		{"MP " + bar(st.MP, maxMP(st.Lvl), 14) + fmt.Sprintf(" %d/%d", st.MP, maxMP(st.Lvl)), RGB{100, 160, 255}},
		{"XP " + bar(st.XP, xpNeed(st.Lvl), 14) + fmt.Sprintf(" %d/%d", st.XP, xpNeed(st.Lvl)), RGB{120, 230, 140}},
		{fmt.Sprintf("Oro: %d  Atq %d  Def %d", st.Gold, st.atk(), st.def()), RGB{255, 210, 80}},
		{"Arma: " + itemName(st.Weapon), RGB{200, 200, 220}},
		{"Armadura: " + itemName(st.Armor), RGB{200, 200, 220}},
		{fmt.Sprintf("Z vida(%d)  X maná(%d)", st.countKind('h'), st.countKind('m')), RGB{170, 200, 170}},
	}
	for i, s := range cl.Specials {
		t := fmt.Sprintf("%d %s", i+1, s.Name)
		c := RGB{200, 210, 235}
		switch {
		case st.Lvl < s.Lvl:
			t, c = fmt.Sprintf("%d (nivel %d)", i+1, s.Lvl), RGB{110, 110, 130}
		case now.Before(st.CD[i]):
			t += fmt.Sprintf(" %.0fs", math.Ceil(st.CD[i].Sub(now).Seconds()))
			c = RGB{140, 140, 160}
		default:
			t += fmt.Sprintf(" (%d MP)", s.MP)
		}
		lines = append(lines, struct {
			t string
			c RGB
		}{t, c})
	}
	bh := len(lines) + 2
	cv.box(0, 0, bw, bh, RGB{6, 10, 30}, RGB{90, 110, 190})
	for i, l := range lines {
		r := []rune(l.t)
		if len(r) > bw-3 {
			r = r[:bw-3]
		}
		cv.text(2, 1+i, string(r), l.c)
	}
	banner := func(t string, c RGB) {
		y := cv.h / 3
		w := len([]rune(t)) + 6
		cv.box((cv.w-w)/2, y-1, w, 3, RGB{20, 15, 40}, c)
		cv.text(-1, y, t, c)
	}
	switch {
	case now.Sub(st.LevelUpAt) < 3*time.Second:
		c := RGB{255, 240, 120}
		if now.UnixMilli()/200%2 == 0 {
			c = RGB{255, 160, 60}
		}
		banner(fmt.Sprintf("✦ ¡SUBISTE DE NIVEL! Nivel %d ✦", st.Lvl), c)
	case now.Sub(st.DiedAt) < 3*time.Second:
		banner("Has caído... despiertas en Lago Lunar", RGB{255, 100, 100})
	case st.Msg != "" && now.Sub(st.MsgAt) < 1500*time.Millisecond:
		cv.text(-1, cv.h-3, " "+st.Msg+" ", RGB{255, 230, 160})
	}
}

func crowded(self *Enemy, x, y int) bool {
	for _, o := range enemies {
		if o != self && !o.Dead && abs(o.X-x) < 9 && abs(o.Y-y) < 3 && abs(o.X-x)+abs(o.Y-y) < abs(o.X-self.X)+abs(o.Y-self.Y) {
			return true
		}
	}
	return false
}
