// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/crypto/bcrypt"
)

// Account es una cuenta guardada en accounts.json. Los campos nuevos van con
// omitempty para que los archivos viejos sigan cargando.
type Account struct {
	User      string            `json:"user"`
	Hash      string            `json:"hash"`
	Created   time.Time         `json:"created"`
	LastLogin time.Time         `json:"last_login,omitempty"`
	Char      *SavedChar        `json:"char,omitempty"`
	X         int               `json:"x"`
	Y         int               `json:"y"`
	HasPos    bool              `json:"has_pos"`
	Quests    map[string]QState `json:"quests,omitempty"`
	Weapon    string            `json:"weapon,omitempty"`
	Armor     string            `json:"armor,omitempty"`
	HP        int               `json:"hp,omitempty"`
	MP        int               `json:"mp,omitempty"`
	Level     int               `json:"level,omitempty"`
	XP        int               `json:"xp,omitempty"`
	Money     int               `json:"money,omitempty"`
	Inventory []Item            `json:"inventory,omitempty"`
	Extra     map[string]any    `json:"extra,omitempty"`
	Admin     bool              `json:"admin,omitempty"`
	Playtime  int64             `json:"playtime_secs,omitempty"`
	LastIP    string            `json:"last_ip,omitempty"`
}

// SavedChar es el personaje elegido: nombre, sprite y colores.
type SavedChar struct {
	Name   string `json:"name"`
	Sprite int    `json:"sprite"`
	Prim   int    `json:"prim"`
	Sec    int    `json:"sec"`
}

// Item es una pila de objetos en el inventario.
type Item struct {
	ID  string `json:"id"`
	Qty int    `json:"qty"`
}

var (
	accMu    sync.Mutex
	accounts = map[string]*Account{} // clave: usuario en minúsculas
	accPath  string
	online   = map[string]bool{}
	fails    = map[string]*failRec{}
)

type failRec struct {
	n     int
	until time.Time
}

var dummyHash = func() string { h, _ := bcrypt.GenerateFromPassword([]byte("x"), bcrypt.DefaultCost); return string(h) }()

func dataPath() string {
	d := os.Getenv("DATA_DIR")
	if d == "" {
		h, _ := os.UserHomeDir()
		d = filepath.Join(h, "ascii-ssh-game", "data")
	}
	return filepath.Join(d, "accounts.json")
}

func loadAccounts() error {
	accMu.Lock()
	defer accMu.Unlock()
	accPath = dataPath()
	if err := os.MkdirAll(filepath.Dir(accPath), 0700); err != nil {
		return err
	}
	b, err := os.ReadFile(accPath)
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	var list []*Account
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	for _, a := range list {
		accounts[strings.ToLower(a.User)] = a
	}
	return nil
}

// saveLocked escribe de forma atómica; hay que tener accMu.
func saveLocked() error {
	list := make([]*Account, 0, len(accounts))
	for _, a := range accounts {
		list = append(list, a)
	}
	b, err := json.MarshalIndent(list, "", " ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(accPath), ".accounts-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Sync()
	tmp.Close()
	os.Chmod(tmp.Name(), 0600)
	return os.Rename(tmp.Name(), accPath)
}

func validUser(u string) bool {
	r := []rune(u)
	if len(r) < 3 || len(r) > 16 {
		return false
	}
	for _, c := range r {
		if !(c == '_' || (c < 128 && (unicode.IsLetter(c) || unicode.IsDigit(c)))) {
			return false
		}
	}
	return true
}

func limited(keys ...string) time.Duration {
	now := time.Now()
	var w time.Duration
	for _, k := range keys {
		if f := fails[k]; f != nil && now.Before(f.until) {
			w = max(w, f.until.Sub(now))
		}
	}
	return w
}
func addFail(keys ...string) {
	for _, k := range keys {
		f := fails[k]
		if f == nil || (!f.until.IsZero() && time.Now().After(f.until)) {
			f = &failRec{}
			fails[k] = f
		}
		f.n++
		if f.n >= 5 {
			f.until = time.Now().Add(time.Minute)
		}
	}
}

func createAccount(user, pass string) (*Account, string) {
	if !validUser(user) {
		return nil, "Usuario: 3-16 letras, números o _"
	}
	if len([]rune(pass)) < 4 {
		return nil, "La contraseña debe tener 4+ caracteres"
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return nil, "Error interno"
	}
	accMu.Lock()
	defer accMu.Unlock()
	k := strings.ToLower(user)
	if accounts[k] != nil {
		return nil, "Ese usuario ya existe"
	}
	a := &Account{User: user, Hash: string(h), Created: time.Now(), LastLogin: time.Now(), Level: 1}
	accounts[k] = a
	if err := saveLocked(); err != nil {
		delete(accounts, k)
		return nil, "No se pudo guardar"
	}
	online[k] = true
	return a, ""
}

func login(user, pass, ip string) (*Account, string) {
	k := strings.ToLower(user)
	accMu.Lock()
	if w := limited("u:"+k, "ip:"+ip); w > 0 {
		accMu.Unlock()
		return nil, "Demasiados intentos. Espera " + itoa(int(math.Ceil(w.Seconds()))) + " s"
	}
	a := accounts[k]
	accMu.Unlock()
	hash := dummyHash // mismo tiempo de respuesta para usuarios que no existen
	if a != nil {
		hash = a.Hash
	}
	ok := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) == nil && a != nil
	accMu.Lock()
	defer accMu.Unlock()
	if !ok {
		addFail("u:"+k, "ip:"+ip)
		return nil, "Usuario o contraseña incorrectos"
	}
	if accBannedLocked(k) {
		return nil, "Esta cuenta está baneada"
	}
	if online[k] {
		return nil, "Esa cuenta ya está conectada"
	}
	delete(fails, "u:"+k)
	online[k] = true
	a.LastLogin = time.Now()
	saveLocked()
	return a, ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// persist guarda personaje y posición en la cuenta. Se puede llamar seguido.
func (m *model) persist() {
	m.am.Lock()
	a := m.acc
	m.am.Unlock()
	if a == nil {
		return
	}
	mu.Lock()
	var x, y int
	var st Stats
	has := m.p != nil
	if has {
		x, y, st = m.p.X, m.p.Y, m.p.St
		st.Inv = append([]Item(nil), st.Inv...)
		st.Quests = copyQuests(st.Quests)
	}
	mu.Unlock()
	accMu.Lock()
	defer accMu.Unlock()
	a.Char = &SavedChar{Name: strings.TrimSpace(string(m.name)), Sprite: m.char, Prim: m.prim, Sec: m.sec}
	if has {
		a.X, a.Y, a.HasPos = x, y, true
		a.Level, a.XP, a.Money, a.HP, a.MP = st.Lvl, st.XP, st.Gold, st.HP, st.MP
		a.Inventory, a.Weapon, a.Armor = append([]Item(nil), st.Inv...), st.Weapon, st.Armor
		a.Quests = st.Quests
	}
	saveLocked()
}

func (m *model) logout() {
	m.am.Lock()
	if a := m.acc; a != nil && !m.loginAt.IsZero() {
		accMu.Lock()
		a.Playtime += int64(time.Since(m.loginAt).Seconds())
		accMu.Unlock()
		m.loginAt = time.Time{}
	}
	m.am.Unlock()
	m.persist()
	m.am.Lock()
	a := m.acc
	m.acc = nil
	m.am.Unlock()
	if a == nil {
		return
	}
	accMu.Lock()
	delete(online, strings.ToLower(a.User))
	accMu.Unlock()
}

// formularios
type form struct {
	labels []string
	vals   [][]rune
	secret []bool
	focus  int
	err    string
}

func newForm(signup bool) *form {
	if signup {
		return &form{labels: []string{"Usuario", "Contraseña", "Confirmar"}, vals: make([][]rune, 3), secret: []bool{false, true, true}}
	}
	return &form{labels: []string{"Usuario", "Contraseña"}, vals: make([][]rune, 2), secret: []bool{false, true}}
}

func (m *model) formKey(msg tea.KeyMsg, signup bool) {
	f := m.form
	switch msg.Type {
	case tea.KeyEsc:
		m.ph, m.form = phTitle, nil
		return
	case tea.KeyTab, tea.KeyDown:
		f.focus = (f.focus + 1) % len(f.vals)
		return
	case tea.KeyShiftTab, tea.KeyUp:
		f.focus = (f.focus + len(f.vals) - 1) % len(f.vals)
		return
	case tea.KeyBackspace:
		if v := f.vals[f.focus]; len(v) > 0 {
			f.vals[f.focus] = v[:len(v)-1]
		}
		return
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if !unicode.IsControl(r) && len(f.vals[f.focus]) < 32 {
				f.vals[f.focus] = append(f.vals[f.focus], r)
			}
		}
		return
	case tea.KeyEnter:
	default:
		return
	}
	if f.focus < len(f.vals)-1 {
		f.focus++
		return
	}
	user, pass := string(f.vals[0]), string(f.vals[1])
	if signup {
		if pass != string(f.vals[2]) {
			f.err, f.vals[1], f.vals[2], f.focus = "Las contraseñas no coinciden", nil, nil, 1
			return
		}
		a, e := createAccount(user, pass)
		if a == nil {
			f.err = e
			return
		}
		m.am.Lock()
		m.acc, m.form = a, nil
		m.am.Unlock()
		onLogin(m, a)
		m.name = []rune(a.User)
		m.ph = phChar
		return
	}
	a, e := login(user, pass, m.remote)
	if a == nil {
		f.err, f.vals[1], f.focus = e, nil, 1
		return
	}
	m.am.Lock()
	m.acc, m.form = a, nil
	m.am.Unlock()
	onLogin(m, a)
	if a.Char == nil {
		m.name = []rune(a.User)
		m.ph = phChar
		return
	}
	m.name = []rune(a.Char.Name)
	m.char, m.prim, m.sec = a.Char.Sprite%len(sprites), a.Char.Prim%len(colors), a.Char.Sec%len(colors)
	m.ph = phWelcome
}

func (m *model) drawForm(cv *canvas, y int, signup bool) {
	f := m.form
	title := "ENTRAR"
	if signup {
		title = "CREAR CUENTA"
	}
	bw, bh := 44, 2*len(f.vals)+7
	y = clamp(min(y, cv.h-bh-1), 0, cv.h)
	x0 := (cv.w - bw) / 2
	cv.box(x0, y, bw, bh, RGB{6, 10, 30}, RGB{90, 110, 190})
	cv.text(-1, y+1, title, RGB{255, 220, 120})
	blink := time.Now().UnixMilli()/500%2 == 0
	for i, l := range f.labels {
		v := string(f.vals[i])
		if f.secret[i] {
			v = strings.Repeat("*", len(f.vals[i]))
		}
		c, mark := RGB{150, 160, 190}, "  "
		if i == f.focus {
			c, mark = RGB{255, 255, 255}, "▸ "
			if blink {
				v += "█"
			}
		}
		cv.text(x0+3, y+3+2*i, mark+padR(l+":", 12)+v, c)
	}
	if f.err != "" {
		cv.text(-1, y+bh-3, f.err, RGB{255, 110, 110})
	}
	cv.text(-1, y+bh-2, "[Tab] campo  [Enter] seguir  [Esc] volver", RGB{130, 140, 180})
}

func padR(s string, n int) string {
	for len([]rune(s)) < n {
		s += " "
	}
	return s
}

func (m *model) drawWelcome(cv *canvas, y int) {
	bw, bh := 44, 7
	y = clamp(min(y, cv.h-bh-1), 0, cv.h)
	cv.box((cv.w-bw)/2, y, bw, bh, RGB{6, 10, 30}, RGB{90, 110, 190})
	cv.text(-1, y+1, "¡Bienvenido de vuelta, "+string(m.name)+"!", RGB{255, 220, 120})
	cv.text(-1, y+3, "[Enter] Jugar", RGB{255, 255, 255})
	cv.text(-1, y+4, "[P] Personalizar personaje", RGB{200, 210, 235})
	cv.text(-1, y+5, "[Esc] Salir de la cuenta", RGB{130, 140, 180})
}
