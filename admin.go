// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

// Área de administración. Son admin solo cuentas con sesión iniciada, nunca invitados:
//   - usuarios en la variable ADMINS (separados por coma, sin importar mayúsculas)
//   - cuentas con "admin": true en accounts.json
//   - la cuenta "xo" si existe; se desactiva con ADMINS_NO_DEFAULT=1
// En el juego el panel se abre con O o Ctrl+A.
// CLI (lee la carpeta de datos, no necesita el servidor):
//   ascii-ssh-game admin online|historial [n]|cuentas|acciones [n]

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
)

var mxTZ = func() *time.Location {
	if l, err := time.LoadLocation("America/Mexico_City"); err == nil {
		return l
	}
	return time.FixedZone("CST", -6*3600)
}()

func mxTime(t time.Time) string { return t.In(mxTZ).Format("2006-01-02 15:04") }

func dataDir() string { return filepath.Dir(dataPath()) }

// quién es admin
func isAdminName(user string) bool {
	u := strings.ToLower(strings.TrimSpace(user))
	if u == "" {
		return false
	}
	for _, a := range strings.Split(os.Getenv("ADMINS"), ",") {
		if strings.ToLower(strings.TrimSpace(a)) == u {
			return true
		}
	}
	return u == "xo" && os.Getenv("ADMINS_NO_DEFAULT") == ""
}

// isAdmin se revisa en el servidor antes de cada acción.
func (m *model) isAdmin() bool {
	m.am.Lock()
	a := m.acc
	m.am.Unlock()
	if a == nil {
		return false
	}
	accMu.Lock()
	defer accMu.Unlock()
	if accounts[strings.ToLower(a.User)] != a { // borrada o vieja
		return false
	}
	return a.Admin || isAdminName(a.User)
}

// registro de sesiones
type sessInfo struct {
	m        *model
	sess     ssh.Session
	ip, term string
	mode     colorMode
	w, h     int
	start    time.Time
	loginAt  time.Time
	user     string // nombre de la cuenta al iniciar sesión
	kicked   bool
}

var (
	regMu    sync.Mutex
	sessions = map[*model]*sessInfo{}
)

func register(m *model, s ssh.Session, term string) {
	regMu.Lock()
	sessions[m] = &sessInfo{m: m, sess: s, ip: m.remote, term: term, mode: m.mode, w: m.w, h: m.h, start: time.Now()}
	regMu.Unlock()
	logSession("conectar", m, 0)
	writeOnline()
}

func unregister(m *model) {
	regMu.Lock()
	si := sessions[m]
	delete(sessions, m)
	regMu.Unlock()
	if si != nil {
		logSession("salir", m, time.Since(si.start))
	}
	writeOnline()
}

func regSize(m *model, w, h int) {
	regMu.Lock()
	if si := sessions[m]; si != nil {
		si.w, si.h = w, h
	}
	regMu.Unlock()
}

// onLogin marca que la sesión ya entró con una cuenta.
func onLogin(m *model, a *Account) {
	regMu.Lock()
	if si := sessions[m]; si != nil {
		si.user, si.loginAt = a.User, time.Now()
	}
	regMu.Unlock()
	m.loginAt = time.Now()
	accMu.Lock()
	a.LastIP = m.remote
	accMu.Unlock()
	logSession("entrar", m, 0)
	writeOnline()
}

// bitácoras
type sessEntry struct {
	T       time.Time `json:"t"`
	Ev      string    `json:"ev"` // conectar, entrar o salir
	User    string    `json:"user"`
	Account bool      `json:"account"`
	IP      string    `json:"ip"`
	Secs    int64     `json:"secs,omitempty"`
}

var logMu sync.Mutex

func appendJSONL(name string, v any) {
	b, _ := json.Marshal(v)
	logMu.Lock()
	defer logMu.Unlock()
	f, err := os.OpenFile(filepath.Join(dataDir(), name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	f.Write(append(b, '\n'))
	f.Close()
}

func (m *model) whoName() (string, bool) {
	m.am.Lock()
	a := m.acc
	m.am.Unlock()
	if a != nil {
		return a.User, true
	}
	mu.Lock()
	defer mu.Unlock()
	if m.p != nil {
		return m.p.Name, false
	}
	return m.user, false
}

func logSession(ev string, m *model, d time.Duration) {
	u, acc := m.whoName()
	appendJSONL("sessions.log", sessEntry{time.Now(), ev, u, acc, m.remote, int64(d.Seconds())})
}

type actEntry struct {
	T      time.Time `json:"t"`
	Admin  string    `json:"admin"`
	Action string    `json:"action"`
	Target string    `json:"target,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

func logAction(admin, action, target, detail string) {
	appendJSONL("admin.log", actEntry{time.Now(), admin, action, target, detail})
}

func tailJSONL[T any](name string, n int) []T {
	f, err := os.Open(filepath.Join(dataDir(), name))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var v T
		if json.Unmarshal(sc.Bytes(), &v) == nil {
			out = append(out, v)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out
}

// baneos y silencios (data/bans.json)
type banList struct {
	Accounts map[string]string `json:"accounts"` // usuario en minúsculas -> motivo
	IPs      map[string]string `json:"ips"`
	Muted    map[string]bool   `json:"muted"` // cuenta o nombre de invitado, en minúsculas
}

var (
	banMu sync.Mutex
	bans  = banList{map[string]string{}, map[string]string{}, map[string]bool{}}
)

func loadBans() {
	banMu.Lock()
	defer banMu.Unlock()
	if b, err := os.ReadFile(filepath.Join(dataDir(), "bans.json")); err == nil {
		json.Unmarshal(b, &bans)
	}
	if bans.Accounts == nil {
		bans.Accounts = map[string]string{}
	}
	if bans.IPs == nil {
		bans.IPs = map[string]string{}
	}
	if bans.Muted == nil {
		bans.Muted = map[string]bool{}
	}
}

func saveBansLocked() {
	b, _ := json.MarshalIndent(bans, "", " ")
	p := filepath.Join(dataDir(), "bans.json")
	os.WriteFile(p+".tmp", b, 0600)
	os.Rename(p+".tmp", p)
}

func ipBanned(ip string) bool {
	banMu.Lock()
	defer banMu.Unlock()
	_, ok := bans.IPs[ip]
	return ok
}
func accBanned(user string) bool {
	banMu.Lock()
	defer banMu.Unlock()
	_, ok := bans.Accounts[strings.ToLower(user)]
	return ok
}
func isMuted(name string) bool {
	banMu.Lock()
	defer banMu.Unlock()
	return bans.Muted[strings.ToLower(name)]
}

// anuncios
type announcement struct {
	Text string
	By   string
	At   time.Time
}

var curAnnounce announcement // protegido por mu

const announceDur = 10 * time.Second

func drawAnnouncement(cv *canvas, a announcement, now time.Time) {
	if a.Text == "" || now.Sub(a.At) > announceDur {
		return
	}
	lines := wrap(a.Text, min(60, cv.w-8))
	w := 8
	for _, l := range lines {
		w = max(w, len([]rune(l))+8)
	}
	w = min(w, cv.w)
	h := len(lines) + 4
	y0 := max(1, cv.h/5)
	x0 := (cv.w - w) / 2
	gold := RGB{255, 210, 80}
	if (now.Sub(a.At)/(400*time.Millisecond))%2 == 0 && now.Sub(a.At) < 2*time.Second {
		gold = RGB{255, 255, 200}
	}
	cv.box(x0, y0, w, h, RGB{40, 10, 10}, gold)
	cv.text(x0+2, y0, " ✦ ANUNCIO DEL SERVIDOR ✦ ", gold)
	for i, l := range lines {
		r := []rune(strings.ToUpper(l))
		x := x0 + (w-len(r))/2
		for j, ch := range r {
			if xx := x + j; xx >= 0 && xx < cv.w && y0+2+i < cv.h {
				cv.c[y0+2+i][xx] = TCell{ch, RGB{255, 255, 255}, RGB{40, 10, 10}}
			}
		}
	}
}

// jugadores en línea; también se escribe data/online.json para la CLI
type onlineRow struct {
	Name, User, Char, Zone, IP, Term string
	Account, Admin, Muted            bool
	Lvl, X, Y, W, H                  int
	Since                            time.Time
	m                                *model
}

func onlineRows() []onlineRow {
	regMu.Lock()
	var infos []sessInfo
	for _, si := range sessions {
		infos = append(infos, *si)
	}
	regMu.Unlock()
	var rows []onlineRow
	for _, si := range infos {
		r := onlineRow{IP: si.ip, Term: fmt.Sprintf("%dx%d %s", si.w, si.h, si.mode), Since: si.start, m: si.m, User: si.user, Account: si.user != ""}
		mu.Lock()
		if p := si.m.p; p != nil {
			r.Name, r.Lvl, r.X, r.Y = p.Name, p.St.Lvl, p.X, p.Y
			r.Char = classOf(p.Sprite).Name
			if z := safeAt(p.X, p.Y); z != nil {
				r.Zone = z.Name
			} else {
				r.Zone = "salvaje"
			}
		} else {
			r.Name, r.Zone = si.m.user, "(menú)"
		}
		mu.Unlock()
		r.Muted = isMuted(r.key())
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Since.Before(rows[j].Since) })
	return rows
}

func (r onlineRow) key() string {
	if r.User != "" {
		return r.User
	}
	return r.Name
}

var onlineMu sync.Mutex

func writeOnline() {
	rows := onlineRows()
	type out struct {
		Updated time.Time   `json:"updated"`
		PID     int         `json:"pid"`
		Players []onlineRow `json:"players"`
	}
	b, _ := json.MarshalIndent(out{time.Now(), os.Getpid(), rows}, "", " ")
	onlineMu.Lock()
	defer onlineMu.Unlock()
	p := filepath.Join(dataDir(), "online.json")
	if os.WriteFile(p+".tmp", b, 0600) == nil {
		os.Rename(p+".tmp", p)
	}
}

func onlineWriter() {
	for range time.Tick(5 * time.Second) {
		writeOnline()
	}
}

func durStr(d time.Duration) string {
	d = d.Round(time.Second)
	h, mnt, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, mnt)
	}
	if mnt > 0 {
		return fmt.Sprintf("%dm%02ds", mnt, s)
	}
	return fmt.Sprintf("%ds", s)
}

// acciones; cada una vuelve a revisar permisos
func (m *model) adminName() string { n, _ := m.whoName(); return n }

func findSession(t *model) *sessInfo {
	regMu.Lock()
	defer regMu.Unlock()
	return sessions[t]
}

func (m *model) actTeleport(t *model) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	mu.Lock()
	defer mu.Unlock()
	if m.p == nil || t.p == nil || t == m {
		return "Ese jugador no está en el mundo"
	}
	x, y := t.p.X+3, t.p.Y
	if blocked(x, y) {
		x = t.p.X
	}
	m.p.X, m.p.Y = x, y
	logAction(m.adminName(), "teleport", t.p.Name, fmt.Sprintf("%d,%d", x, y))
	return "Teletransportado junto a " + t.p.Name
}

func (m *model) actKick(t *model, msg string) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	if t == m {
		return "No puedes expulsarte a ti mismo"
	}
	si := findSession(t)
	if si == nil {
		return "Ya no está conectado"
	}
	if msg == "" {
		msg = "Has sido expulsado por un administrador."
	}
	name, _ := t.whoName()
	logAction(m.adminName(), "kick", name, msg)
	kickSession(si, msg)
	return "Expulsado: " + name
}

func kickSession(si *sessInfo, msg string) {
	regMu.Lock()
	si.kicked = true
	regMu.Unlock()
	if si.m.rr != nil {
		si.m.rr.stop()
	}
	if si.m.out != nil {
		si.m.out.Write([]byte("\x1b[?1049l\x1b[2J\x1b[H\r\n  LAGO LUNAR\r\n\r\n  " + msg + "\r\n\r\n"))
	}
	if si.sess != nil {
		si.sess.Exit(1)
		si.sess.Close()
	}
}

func (m *model) actAnnounce(text string) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	text = sanitize(text)
	if text == "" {
		return "Anuncio vacío"
	}
	mu.Lock()
	curAnnounce = announcement{text, m.adminName(), time.Now()}
	chatLog = append(chatLog, chatLine{"[ANUNCIO]", text, nil})
	mu.Unlock()
	logAction(m.adminName(), "anuncio", "", text)
	return "Anuncio enviado"
}

func (m *model) actMute(key string) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	banMu.Lock()
	k := strings.ToLower(key)
	on := !bans.Muted[k]
	if on {
		bans.Muted[k] = true
	} else {
		delete(bans.Muted, k)
	}
	saveBansLocked()
	banMu.Unlock()
	if on {
		logAction(m.adminName(), "silenciar", key, "")
		return key + " silenciado"
	}
	logAction(m.adminName(), "quitar silencio", key, "")
	return key + " puede hablar de nuevo"
}

// actBan banea o desbanea una cuenta (kind 'a') o una IP (kind 'i') y saca a quien coincida.
func (m *model) actBan(kind byte, key string) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	if key == "" {
		return "Nada que banear"
	}
	banMu.Lock()
	mp, label := bans.Accounts, "cuenta"
	k := strings.ToLower(key)
	if kind == 'i' {
		mp, label, k = bans.IPs, "IP", key
	}
	_, was := mp[k]
	if was {
		delete(mp, k)
	} else {
		mp[k] = "baneado por " + m.adminName() + " " + mxTime(time.Now())
	}
	saveBansLocked()
	banMu.Unlock()
	if was {
		logAction(m.adminName(), "desbanear "+label, key, "")
		return label + " desbaneada: " + key
	}
	logAction(m.adminName(), "banear "+label, key, "")
	regMu.Lock()
	var hit []*sessInfo
	for t, si := range sessions {
		if t != m && ((kind == 'i' && si.ip == key) || (kind == 'a' && strings.EqualFold(si.user, key))) {
			hit = append(hit, si)
		}
	}
	regMu.Unlock()
	for _, si := range hit {
		kickSession(si, "Has sido baneado de este servidor.")
	}
	return fmt.Sprintf("%s baneada: %s (%d sesión(es) cerradas)", label, key, len(hit))
}

func (m *model) actResetLock(user, ip string) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	accMu.Lock()
	n := 0
	for _, k := range []string{"u:" + strings.ToLower(user), "ip:" + ip} {
		if _, ok := fails[k]; ok {
			delete(fails, k)
			n++
		}
	}
	accMu.Unlock()
	logAction(m.adminName(), "reset bloqueo", user, ip)
	return fmt.Sprintf("Bloqueo de intentos reiniciado (%d)", n)
}

// panel
type adminPanel struct {
	tab    int // 0 En línea, 1 Historial, 2 Cuentas, 3 Acciones
	sel    int
	scroll int
	search []rune
	typing bool   // escribiendo en el buscador
	prompt string // "kick" o "anuncio" mientras se pide texto
	input  []rune
	msg    string
	msgAt  time.Time
	target *model // jugador elegido para las acciones
}

var adminTabs = []string{"1 En línea", "2 Historial", "3 Cuentas", "4 Acciones"}

func (m *model) openAdmin() {
	if !m.isAdmin() {
		return
	}
	if m.adm == nil {
		m.adm = &adminPanel{}
	}
	m.win, m.dx, m.dy = 6, 0, 0
	logAction(m.adminName(), "abrir panel", "", "")
}

type accRow struct {
	User, Char, IP        string
	Lvl, Gold             int
	Created, Last         time.Time
	Play                  int64
	Admin, Banned, Online bool
	Locked                bool
}

func accountRows(filter string) []accRow {
	filter = strings.ToLower(filter)
	accMu.Lock()
	var rows []accRow
	for k, a := range accounts {
		if filter != "" && !strings.Contains(k, filter) && !(a.Char != nil && strings.Contains(strings.ToLower(a.Char.Name), filter)) {
			continue
		}
		r := accRow{User: a.User, Lvl: max(1, a.Level), Gold: a.Money, Created: a.Created, Last: a.LastLogin, Play: a.Playtime, IP: a.LastIP, Admin: a.Admin || isAdminName(a.User), Online: online[k]}
		if a.Char != nil {
			r.Char = a.Char.Name
		}
		if f := fails["u:"+k]; f != nil && time.Now().Before(f.until) {
			r.Locked = true
		}
		rows = append(rows, r)
	}
	accMu.Unlock()
	for i := range rows {
		rows[i].Banned = accBanned(rows[i].User)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Last.After(rows[j].Last) })
	return rows
}

func (m *model) adminKey(msg tea.KeyMsg) {
	a := m.adm
	if a == nil || !m.isAdmin() {
		m.win = 0
		return
	}
	k := msg.String()
	setMsg := func(s string) { a.msg, a.msgAt = s, time.Now() }
	if a.prompt != "" || a.typing {
		switch msg.Type {
		case tea.KeyEsc:
			a.prompt, a.input, a.typing = "", nil, false
		case tea.KeyEnter:
			if a.typing {
				a.typing = false
				return
			}
			txt := string(a.input)
			switch a.prompt {
			case "kick":
				if a.target != nil {
					setMsg(m.actKick(a.target, sanitize(txt)))
				}
			case "anuncio":
				setMsg(m.actAnnounce(txt))
			}
			a.prompt, a.input = "", nil
		case tea.KeyBackspace:
			if a.typing && len(a.search) > 0 {
				a.search = a.search[:len(a.search)-1]
				a.sel = 0
			} else if len(a.input) > 0 {
				a.input = a.input[:len(a.input)-1]
			}
		case tea.KeyRunes, tea.KeySpace:
			rs := []rune(sanitizeKeep(msg.Runes))
			if msg.Type == tea.KeySpace {
				rs = []rune{' '}
			}
			if a.typing {
				if len(a.search) < 20 {
					a.search = append(a.search, rs...)
				}
				a.sel = 0
			} else if len(a.input) < 120 {
				a.input = append(a.input, rs...)
			}
		}
		return
	}
	switch k {
	case "esc", "o", "O", "ctrl+a", "q":
		m.win = 0
		return
	case "1", "2", "3", "4":
		a.tab, a.sel, a.scroll = int(k[0]-'1'), 0, 0
		return
	case "tab", "right":
		a.tab, a.sel, a.scroll = (a.tab+1)%4, 0, 0
		return
	case "shift+tab", "left":
		a.tab, a.sel, a.scroll = (a.tab+3)%4, 0, 0
		return
	case "up", "w":
		a.sel = max(0, a.sel-1)
		return
	case "down", "s":
		a.sel++
		return
	case "pgup":
		a.sel = max(0, a.sel-10)
		return
	case "pgdown":
		a.sel += 10
		return
	case "home":
		a.sel = 0
		return
	case "end":
		a.sel = 1 << 20
		return
	}
	switch a.tab {
	case 0, 3: // las acciones sobre jugadores funcionan en En línea y Acciones
		rows := onlineRows()
		if len(rows) == 0 {
			return
		}
		a.sel = min(a.sel, len(rows)-1)
		r := rows[a.sel]
		a.target = r.m
		switch k {
		case "enter", "t", "T":
			setMsg(m.actTeleport(r.m))
		case "k", "K":
			a.prompt, a.input = "kick", nil
		case "m", "M":
			setMsg(m.actMute(r.key()))
		case "b":
			if r.Account {
				setMsg(m.actBan('a', r.User))
			} else {
				setMsg("Invitado: usa B para banear su IP")
			}
		case "B":
			setMsg(m.actBan('i', r.IP))
		case "r", "R":
			setMsg(m.actResetLock(r.key(), r.IP))
		case "a", "A":
			a.prompt, a.input = "anuncio", nil
		}
		if a.tab == 0 && (k == "a" || k == "A") {
			a.prompt = "anuncio"
		}
	case 2:
		if k == "/" || k == "f" || k == "F" {
			a.typing = true
			return
		}
		rows := accountRows(string(a.search))
		if len(rows) == 0 {
			return
		}
		a.sel = min(a.sel, len(rows)-1)
		r := rows[a.sel]
		switch k {
		case "b":
			setMsg(m.actBan('a', r.User))
		case "B":
			setMsg(m.actBan('i', r.IP))
		case "m", "M":
			setMsg(m.actMute(r.User))
		case "r", "R":
			setMsg(m.actResetLock(r.User, r.IP))
		case "x", "X":
			setMsg(m.actToggleAdmin(r.User))
		}
	}
	if a.tab == 3 && (k == "a" || k == "A") {
		a.prompt, a.input = "anuncio", nil
	}
}

func (m *model) actToggleAdmin(user string) string {
	if !m.isAdmin() {
		return "Sin permiso"
	}
	accMu.Lock()
	a := accounts[strings.ToLower(user)]
	if a == nil {
		accMu.Unlock()
		return "No existe"
	}
	if isAdminName(a.User) {
		accMu.Unlock()
		return a.User + " es admin por ADMINS/predeterminado"
	}
	a.Admin = !a.Admin
	on := a.Admin
	saveLocked()
	accMu.Unlock()
	logAction(m.adminName(), fmt.Sprintf("admin=%v", on), user, "")
	if on {
		return user + " ahora es admin"
	}
	return user + " ya no es admin"
}

func (m *model) drawAdmin(cv *canvas) {
	a := m.adm
	if a == nil {
		return
	}
	bw, bh := min(cv.w-2, 118), min(cv.h-2, 34)
	x0, y0 := (cv.w-bw)/2, (cv.h-bh)/2
	bg := RGB{8, 10, 22}
	acc := RGB{255, 120, 90}
	cv.box(x0, y0, bw, bh, bg, acc)
	put := func(x, y int, s string, c RGB) {
		for i, ch := range []rune(s) {
			if xx := x0 + x + i; xx > x0 && xx < x0+bw-1 && y > 0 && y < bh-1 && y0+y < cv.h && xx < cv.w {
				cv.c[y0+y][xx] = TCell{ch, c, cv.c[y0+y][xx].Bg}
			}
		}
	}
	hi := func(y int) {
		for x := x0 + 1; x < x0+bw-1 && x < cv.w; x++ {
			if y0+y < cv.h {
				cv.c[y0+y][x].Bg = RGB{40, 50, 90}
			}
		}
	}
	cv.text(x0+2, y0, " ⚙ PANEL DE ADMINISTRACIÓN ", acc)
	tx := 3
	for i, t := range adminTabs {
		c := RGB{150, 150, 170}
		s := " " + t + " "
		if i == a.tab {
			c, s = RGB{255, 230, 120}, "["+t+"]"
		}
		put(tx, 1, s, c)
		tx += len([]rune(s)) + 2
	}
	dim, wt, hd := RGB{150, 160, 180}, RGB{230, 230, 240}, RGB{120, 200, 255}
	listH := bh - 7
	scrollTo := func(n int) int {
		a.sel = max(0, min(a.sel, n-1))
		if a.sel < a.scroll {
			a.scroll = a.sel
		}
		if a.sel >= a.scroll+listH {
			a.scroll = a.sel - listH + 1
		}
		return a.scroll
	}
	now := time.Now()
	foot := ""
	switch a.tab {
	case 0, 3:
		rows := onlineRows()
		put(2, 3, fmt.Sprintf("%-14s %-12s %-9s %3s %-11s %-14s %-15s %-8s %s", "Jugador", "Cuenta", "Clase", "Nv", "Posición", "Zona", "IP", "Tiempo", "Terminal"), hd)
		off := scrollTo(len(rows))
		for i := off; i < len(rows) && i-off < listH; i++ {
			r := rows[i]
			y := 4 + i - off
			acct := "invitado"
			if r.Account {
				acct = r.User
			}
			flags := ""
			if r.Muted {
				flags += " 🔇"
			}
			if i == a.sel {
				hi(y)
			}
			c := wt
			if r.m == m {
				c = RGB{140, 255, 170}
			}
			put(2, y, fmt.Sprintf("%-14.14s %-12.12s %-9.9s %3d %-11s %-14.14s %-15.15s %-8s %s%s", r.Name, acct, r.Char, r.Lvl, fmt.Sprintf("%d,%d", r.X, r.Y), r.Zone, r.IP, durStr(now.Sub(r.Since)), r.Term, flags), c)
		}
		if len(rows) == 0 {
			put(2, 4, "Nadie conectado.", dim)
		}
		if a.tab == 3 {
			put(2, bh-6, "Acciones sobre el jugador seleccionado:", hd)
		}
		foot = "T ir · K expulsar · M silenciar · b ban cuenta · B ban IP · R desbloquear · A anuncio"
		put(2, bh-3, fmt.Sprintf("Conectados: %d", len(rows)), dim)
	case 1:
		ents := tailJSONL[sessEntry]("sessions.log", 200)
		put(2, 3, fmt.Sprintf("%-17s %-9s %-16s %-9s %-15s %s", "Hora (CDMX)", "Evento", "Usuario", "Tipo", "IP", "Duración"), hd)
		n := len(ents)
		off := scrollTo(n)
		for i := off; i < n && i-off < listH; i++ {
			e := ents[n-1-i] // más recientes primero
			y := 4 + i - off
			if i == a.sel {
				hi(y)
			}
			tp := "invitado"
			if e.Account {
				tp = "cuenta"
			}
			d := ""
			if e.Ev == "salir" {
				d = durStr(time.Duration(e.Secs) * time.Second)
			}
			c := wt
			switch e.Ev {
			case "salir":
				c = RGB{255, 170, 150}
			case "entrar":
				c = RGB{150, 255, 170}
			}
			put(2, y, fmt.Sprintf("%-17s %-9s %-16.16s %-9s %-15.15s %s", mxTime(e.T), e.Ev, e.User, tp, e.IP, d), c)
		}
		foot = "↑↓ RePág AvPág desplazar · recientes primero · últimas 200"
		put(2, bh-3, fmt.Sprintf("Entradas: %d", n), dim)
	case 2:
		rows := accountRows(string(a.search))
		sb := "Buscar (/): " + string(a.search)
		if a.typing {
			sb += "▏"
		}
		put(bw-36, 1, sb, RGB{255, 230, 120})
		put(2, 3, fmt.Sprintf("%-16s %-12s %3s %7s %-16s %-16s %-8s %-15s %s", "Cuenta", "Personaje", "Nv", "Oro", "Creada", "Último acceso", "Jugado", "Última IP", "Estado"), hd)
		off := scrollTo(len(rows))
		for i := off; i < len(rows) && i-off < listH; i++ {
			r := rows[i]
			y := 4 + i - off
			if i == a.sel {
				hi(y)
			}
			st := []string{}
			if r.Online {
				st = append(st, "en línea")
			}
			if r.Admin {
				st = append(st, "ADMIN")
			}
			if r.Banned {
				st = append(st, "BANEADA")
			}
			if r.Locked {
				st = append(st, "bloqueada")
			}
			if isMuted(r.User) {
				st = append(st, "silenciada")
			}
			last := "—"
			if !r.Last.IsZero() {
				last = mxTime(r.Last)
			}
			c := wt
			if r.Banned {
				c = RGB{255, 120, 120}
			}
			put(2, y, fmt.Sprintf("%-16.16s %-12.12s %3d %7d %-16s %-16s %-8s %-15.15s %s", r.User, r.Char, r.Lvl, r.Gold, mxTime(r.Created), last, durStr(time.Duration(r.Play)*time.Second), r.IP, strings.Join(st, ",")), c)
		}
		foot = "/ buscar · b ban cuenta · B ban IP · M silenciar · R desbloquear · X admin"
		put(2, bh-3, fmt.Sprintf("Cuentas: %d", len(rows)), dim)
	}
	if a.prompt != "" {
		lbl := "Mensaje de expulsión: "
		if a.prompt == "anuncio" {
			lbl = "Anuncio para todos: "
		}
		put(2, bh-4, lbl+string(a.input)+"▏  (Enter enviar · Esc cancelar)", RGB{255, 230, 120})
	} else if a.msg != "" && now.Sub(a.msgAt) < 6*time.Second {
		put(2, bh-4, "» "+a.msg, RGB{140, 255, 170})
	}
	put(2, bh-2, foot+" · Tab pestañas · Esc cerrar", dim)
}

// CLI: ascii-ssh-game admin ...
func adminCLI(args []string) int {
	if len(args) == 0 {
		args = []string{"online"}
	}
	n := 50
	if len(args) > 1 {
		fmt.Sscanf(args[1], "%d", &n)
	}
	switch args[0] {
	case "online", "enlinea":
		b, err := os.ReadFile(filepath.Join(dataDir(), "online.json"))
		if err != nil {
			fmt.Println("No hay datos (¿el servidor está corriendo?):", err)
			return 1
		}
		var o struct {
			Updated time.Time
			Players []onlineRow
		}
		json.Unmarshal(b, &o)
		age := time.Since(o.Updated)
		fmt.Printf("En línea: %d   (actualizado %s, hace %s)\n", len(o.Players), mxTime(o.Updated), durStr(age))
		if age > 30*time.Second {
			fmt.Println("¡Aviso! datos viejos: el servidor quizá no está corriendo.")
		}
		fmt.Printf("%-14s %-12s %-9s %3s %-11s %-14s %-15s %-8s %s\n", "JUGADOR", "CUENTA", "CLASE", "NV", "POS", "ZONA", "IP", "TIEMPO", "TERMINAL")
		for _, r := range o.Players {
			acct := "invitado"
			if r.Account {
				acct = r.User
			}
			fmt.Printf("%-14.14s %-12.12s %-9.9s %3d %-11s %-14.14s %-15.15s %-8s %s\n", r.Name, acct, r.Char, r.Lvl, fmt.Sprintf("%d,%d", r.X, r.Y), r.Zone, r.IP, durStr(o.Updated.Sub(r.Since)), r.Term)
		}
	case "historial", "history":
		ents := tailJSONL[sessEntry]("sessions.log", n)
		fmt.Printf("%-17s %-9s %-16s %-9s %-15s %s\n", "HORA (CDMX)", "EVENTO", "USUARIO", "TIPO", "IP", "DURACIÓN")
		for _, e := range ents {
			tp, d := "invitado", ""
			if e.Account {
				tp = "cuenta"
			}
			if e.Ev == "salir" {
				d = durStr(time.Duration(e.Secs) * time.Second)
			}
			fmt.Printf("%-17s %-9s %-16.16s %-9s %-15.15s %s\n", mxTime(e.T), e.Ev, e.User, tp, e.IP, d)
		}
	case "cuentas", "accounts":
		if err := loadAccounts(); err != nil {
			fmt.Println("error:", err)
			return 1
		}
		loadBans()
		rows := accountRows("")
		fmt.Printf("%-16s %-12s %3s %7s %-16s %-16s %-8s %s\n", "CUENTA", "PERSONAJE", "NV", "ORO", "CREADA", "ÚLTIMO ACCESO", "JUGADO", "ESTADO")
		for _, r := range rows {
			st := ""
			if r.Admin {
				st += "ADMIN "
			}
			if r.Banned {
				st += "BANEADA"
			}
			fmt.Printf("%-16.16s %-12.12s %3d %7d %-16s %-16s %-8s %s\n", r.User, r.Char, r.Lvl, r.Gold, mxTime(r.Created), mxTime(r.Last), durStr(time.Duration(r.Play)*time.Second), st)
		}
	case "acciones", "actions", "log":
		for _, e := range tailJSONL[actEntry]("admin.log", n) {
			fmt.Printf("%s  %-12s %-18s %-14s %s\n", mxTime(e.T), e.Admin, e.Action, e.Target, e.Detail)
		}
	default:
		fmt.Println("uso: ascii-ssh-game admin [online | historial [n] | cuentas | acciones [n]]")
		fmt.Println("datos en:", dataDir())
		return 2
	}
	return 0
}

// accBannedLocked se llama con accMu tomado; bans tiene su propio candado.
func accBannedLocked(k string) bool { return accBanned(k) }
