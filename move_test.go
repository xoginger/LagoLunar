// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBurstStops(t *testing.T) {
	genMap()
	os.Setenv("SPAWN", "1060,590")
	m := &model{w: 100, h: 30, mode: cm256, user: "T", name: []rune("T")}
	m.join()
	stop := make(chan bool)
	go func() { // tick como el programa real (18 fps)
		for {
			select {
			case <-stop:
				return
			case now := <-time.After(time.Second / 18):
				m.Update(tick(now))
			}
		}
	}()
	x0 := m.p.X
	for i := 0; i < 30; i++ { // ráfaga de repetición cada 33 ms (~30 Hz)
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		time.Sleep(33 * time.Millisecond)
	}
	xRel := m.p.X
	time.Sleep(200 * time.Millisecond)
	xAfter := m.p.X
	time.Sleep(1 * time.Second)
	xLate := m.p.X
	close(stop)
	t.Logf("start=%d release=%d +200ms=%d +1.2s=%d steps=%d", x0, xRel, xAfter, xLate, (xRel-x0)/2)
	if xLate-xRel > 4 {
		t.Fatalf("kept walking after release: %d extra cols", xLate-xRel)
	}
}
