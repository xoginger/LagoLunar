// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"strings"
	"time"
	"unicode"
)

const (
	chatMax    = 80
	bubbleLife = 7 * time.Second
	bubbleW    = 26
)

type chatLine struct {
	Name, Text string
	Party      *Party
}

var chatLog []chatLine // protegido por mu

// sanitize quita caracteres de control y secuencias de escape, y limita el largo.
func sanitize(s string) string {
	var out []rune
	for _, r := range s {
		if r == '\x1b' || unicode.IsControl(r) || r == unicode.ReplacementChar {
			continue
		}
		if unicode.IsSpace(r) {
			r = ' '
		}
		out = append(out, r)
		if len(out) >= chatMax {
			break
		}
	}
	return strings.TrimSpace(string(out))
}

func say(p *Player, text string) {
	text = sanitize(text)
	if text == "" || p == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if isMuted(p.MuteKey) || isMuted(p.Name) {
		p.say2("Un administrador te silenció")
		return
	}
	if strings.HasPrefix(text, "/e ") || text == "/e" { // chat de equipo
		t := strings.TrimSpace(strings.TrimPrefix(text, "/e"))
		if p.Party == nil {
			p.say2("No estás en un equipo")
			return
		}
		if t == "" {
			return
		}
		p.Msg, p.MsgAt, p.MsgParty = t, time.Now(), true
		chatLog = append(chatLog, chatLine{p.Name, t, p.Party})
	} else {
		p.Msg, p.MsgAt, p.MsgParty = text, time.Now(), false
		chatLog = append(chatLog, chatLine{p.Name, text, nil})
	}
	if len(chatLog) > 40 {
		chatLog = chatLog[len(chatLog)-40:]
	}
}

func wrap(s string, w int) []string {
	var lines []string
	cur := []rune{}
	for _, word := range strings.Fields(s) {
		wr := []rune(word)
		for len(wr) > w {
			if len(cur) > 0 {
				lines, cur = append(lines, string(cur)), nil
			}
			lines, wr = append(lines, string(wr[:w])), wr[w:]
		}
		if len(cur) > 0 && len(cur)+1+len(wr) > w {
			lines, cur = append(lines, string(cur)), nil
		}
		if len(cur) > 0 {
			cur = append(cur, ' ')
		}
		cur = append(cur, wr...)
	}
	if len(cur) > 0 {
		lines = append(lines, string(cur))
	}
	return lines
}

// drawBubble dibuja un globo de diálogo con la punta en (cx, tipY). Los de
// equipo llevan borde cian y solo los ve el equipo.
func (cv *canvas) drawBubble(cx, tipY int, text string, party bool) {
	lines := wrap(text, bubbleW-4)
	iw := 0
	for _, l := range lines {
		iw = max(iw, len([]rune(l)))
	}
	bw, bh := iw+4, len(lines)+2
	x0 := clamp(cx-bw/2, 0, max(0, cv.w-bw))
	y0 := tipY - bh
	if y0 < 0 && tipY >= -1 { // si quien habla está arriba, el globo no se sale
		y0 = 0
	}
	bg, border, fg := RGB{245, 245, 235}, RGB{60, 60, 80}, RGB{20, 20, 30}
	if party {
		bg, border, fg = RGB{225, 250, 255}, RGB{40, 200, 230}, RGB{10, 40, 60}
	}
	cv.box(x0, y0, bw, bh, bg, border)
	for i, l := range lines {
		y := y0 + 1 + i
		for j, ch := range []rune(l) {
			if x := x0 + 2 + j; x >= 0 && y >= 0 && x < cv.w && y < cv.h {
				cv.c[y][x] = TCell{ch, fg, bg}
			}
		}
	}
	// punta
	tx := clamp(cx, x0+1, x0+bw-2)
	if y := y0 + bh - 1; y >= 0 && y < cv.h && tx >= 0 && tx < cv.w {
		cv.c[y][tx] = TCell{'┬', border, bg}
	}
	if tipY >= y0+bh && tipY < cv.h && tx >= 0 && tx < cv.w {
		cv.c[tipY][tx].Ch, cv.c[tipY][tx].Fg = '▼', bg
	}
}

func sanitizeKeep(rs []rune) string {
	var out []rune
	for _, r := range rs {
		if r == '\x1b' || unicode.IsControl(r) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}
