// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func adminEnv(t *testing.T) {
	setupWorld(t)
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("ADMINS", "Jefa, otro")
	accMu.Lock()
	accounts = map[string]*Account{}
	online = map[string]bool{}
	fails = map[string]*failRec{}
	accMu.Unlock()
	loadAccounts()
	bans = banList{map[string]string{}, map[string]string{}, map[string]bool{}}
}

func mkSess(t *testing.T, user string, acct bool, ip string) *model {
	m := &model{w: 120, h: 40, mode: cm256, user: user, name: []rune(user), remote: ip}
	register(m, nil, "xterm")
	if acct {
		a, e := createAccount(user, "secreto1")
		if a == nil && len(user) < 3 { // nombre corto heredado como "xo": se mete directo
			a = &Account{User: user, Created: time.Now(), Level: 1}
			accMu.Lock()
			accounts[strings.ToLower(user)], online[strings.ToLower(user)] = a, true
			accMu.Unlock()
		} else if a == nil {
			t.Fatal(e)
		}
		m.acc = a
		onLogin(m, a)
	}
	m.join()
	t.Cleanup(func() { m.leave(); m.logout(); unregister(m) })
	return m
}

func TestAdminIdentity(t *testing.T) {
	adminEnv(t)
	xo := mkSess(t, "xo", true, "1.1.1.1")
	jefa := mkSess(t, "jefa", true, "1.1.1.2")
	pepe := mkSess(t, "pepe", true, "1.1.1.3")
	guest := mkSess(t, "xo2", false, "1.1.1.4")
	if !xo.isAdmin() || !jefa.isAdmin() || pepe.isAdmin() || guest.isAdmin() {
		t.Fatal("admin detection wrong")
	}
	// un invitado que se conecta como "xo" no es admin
	g2 := mkSess(t, "xo", false, "1.1.1.5")
	if g2.isAdmin() {
		t.Fatal("guest named xo is admin")
	}
	t.Setenv("ADMINS_NO_DEFAULT", "1")
	if xo.isAdmin() {
		t.Fatal("default xo should be disabled")
	}
	os.Unsetenv("ADMINS_NO_DEFAULT")
	accMu.Lock()
	pepe.acc.Admin = true
	accMu.Unlock()
	if !pepe.isAdmin() {
		t.Fatal("json admin flag ignored")
	}
}

func TestAdminActionsRequireAdmin(t *testing.T) {
	adminEnv(t)
	pepe := mkSess(t, "pepe", true, "2.2.2.1")
	vic := mkSess(t, "vic", true, "2.2.2.2")
	pepe.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}})
	if pepe.win == 6 {
		t.Fatal("non-admin opened panel")
	}
	for _, r := range []string{pepe.actMute("vic"), pepe.actBan('a', "vic"), pepe.actAnnounce("hola"), pepe.actTeleport(vic), pepe.actKick(vic, "")} {
		if r != "Sin permiso" {
			t.Fatalf("non-admin action allowed: %q", r)
		}
	}
	if isMuted("vic") || accBanned("vic") {
		t.Fatal("state changed")
	}
}

func TestAdminPanelAndActions(t *testing.T) {
	adminEnv(t)
	xo := mkSess(t, "xo", true, "3.3.3.1")
	vic := mkSess(t, "vic", true, "3.3.3.2")
	mkSess(t, "invitado1", false, "3.3.3.3")
	xo.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}})
	if xo.win != 6 {
		t.Fatal("admin panel did not open")
	}
	rows := onlineRows()
	if len(rows) != 3 || !rows[0].Account || rows[2].Account {
		t.Fatalf("online rows %+v", rows)
	}
	if r := xo.actTeleport(vic); !strings.Contains(r, "Teletransportado") {
		t.Fatal(r)
	}
	mu.Lock()
	if abs(xo.p.X-vic.p.X) > 3 || xo.p.Y != vic.p.Y {
		t.Fatal("not teleported")
	}
	mu.Unlock()
	xo.actMute("vic")
	say(vic.p, "hola a todos")
	mu.Lock()
	for _, l := range chatLog {
		if l.Text == "hola a todos" {
			t.Fatal("muted player spoke")
		}
	}
	mu.Unlock()
	xo.actAnnounce("Reinicio en 5 minutos")
	mu.Lock()
	if curAnnounce.Text != "Reinicio en 5 minutos" {
		t.Fatal("no announcement")
	}
	mu.Unlock()
	// desbloqueo
	accMu.Lock()
	for i := 0; i < 6; i++ {
		addFail("u:vic", "ip:3.3.3.2")
	}
	accMu.Unlock()
	xo.actResetLock("vic", "3.3.3.2")
	accMu.Lock()
	if limited("u:vic", "ip:3.3.3.2") > 0 {
		t.Fatal("lockout not reset")
	}
	accMu.Unlock()
	// cuenta baneada: no entra y el baneo se guarda
	xo.actBan('a', "vic")
	vic.leave()
	vic.logout()
	if a, e := login("vic", "secreto1", "9.9.9.9"); a != nil || !strings.Contains(e, "baneada") {
		t.Fatalf("banned login: %v %q", a, e)
	}
	bans = banList{}
	loadBans()
	if !accBanned("vic") {
		t.Fatal("ban not persisted")
	}
	xo.actBan('i', "6.6.6.6")
	if !ipBanned("6.6.6.6") {
		t.Fatal("ip ban")
	}
	// bitácoras
	time.Sleep(10 * time.Millisecond)
	acts := tailJSONL[actEntry]("admin.log", 100)
	if len(acts) < 6 {
		t.Fatalf("actions logged: %d", len(acts))
	}
	ss := tailJSONL[sessEntry]("sessions.log", 100)
	if len(ss) < 4 {
		t.Fatalf("sessions logged: %d", len(ss))
	}
	// dibujar todas las pestañas
	for tab := 0; tab < 4; tab++ {
		xo.adm.tab = tab
		cv := xo.gameCanvas(120, 40)
		var sb strings.Builder
		for _, row := range cv.c {
			for _, c := range row {
				sb.WriteRune(c.Ch)
			}
		}
		if !strings.Contains(sb.String(), "PANEL DE ADMINISTRACI") {
			t.Fatal("panel not drawn")
		}
	}
	writeOnline()
	if adminCLI([]string{"online"}) != 0 || adminCLI([]string{"historial"}) != 0 || adminCLI([]string{"acciones"}) != 0 {
		t.Fatal("cli")
	}
}

func TestPlaytimeTracked(t *testing.T) {
	adminEnv(t)
	m := mkSess(t, "jugon", true, "4.4.4.4")
	m.loginAt = time.Now().Add(-90 * time.Second)
	m.leave()
	m.logout()
	accMu.Lock()
	a := accounts["jugon"]
	accMu.Unlock()
	if a.Playtime < 89 || a.LastIP != "4.4.4.4" || a.LastLogin.IsZero() {
		t.Fatalf("playtime %d ip %q", a.Playtime, a.LastIP)
	}
}

// ADMINSHOT=archivo.ans go test -run AdminShot dibuja el panel para una captura.
func TestAdminShot(t *testing.T) {
	out := os.Getenv("ADMINSHOT")
	if out == "" {
		t.Skip()
	}
	adminEnv(t)
	xo := mkSess(t, "xo", true, "189.203.11.40")
	for i, n := range []string{"luna_23", "Tlaloc", "kiri"} {
		mkSess(t, n, i != 2, []string{"201.141.7.9", "187.190.2.15", "177.228.30.4"}[i])
	}
	xo.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'O'}})
	xo.adm.msg, xo.adm.msgAt = "Teletransportado junto a Tlaloc", time.Now()
	tab := 0
	if v := os.Getenv("TAB"); v != "" {
		tab = int(v[0] - '0')
	}
	xo.adm.tab = tab
	cv := xo.gameCanvas(130, 40)
	os.WriteFile(out, []byte("\x1b[H"+strings.ReplaceAll(cv.String(), "\n", "\r\n")), 0o644)
}
