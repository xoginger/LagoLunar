// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"bytes"
	"io"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// códigos de color
// code: -1 = color por defecto; en 256/16 es índice de paleta; en truecolor 0xRRGGBB.

var lut256 [32 * 32 * 32]uint8
var lut16 [32 * 32 * 32]uint8

func init() {
	for r := 0; r < 32; r++ {
		for g := 0; g < 32; g++ {
			for b := 0; b < 32; b++ {
				c := RGB{float64(r*8 + 4), float64(g*8 + 4), float64(b*8 + 4)}
				lut256[r<<10|g<<5|b] = uint8(to256(c))
				lut16[r<<10|g<<5|b] = uint8(to16(c))
			}
		}
	}
}

func q5(v float64) int { return clamp(int(v), 0, 255) >> 3 }

func (m colorMode) code(c RGB) int32 {
	if c.R < 0 {
		return -1
	}
	switch m {
	case cmTrue:
		return int32(cl(c.R)<<16 | cl(c.G)<<8 | cl(c.B))
	case cm256:
		return int32(lut256[q5(c.R)<<10|q5(c.G)<<5|q5(c.B)])
	default:
		return int32(lut16[q5(c.R)<<10|q5(c.G)<<5|q5(c.B)])
	}
}

// appendSGR agrega el parámetro SGR (sin ESC[ ni m) de un color.
func (m colorMode) appendSGR(b []byte, code int32, bg bool) []byte {
	if code < 0 {
		if bg {
			return append(b, "49"...)
		}
		return append(b, "39"...)
	}
	switch m {
	case cmTrue:
		if bg {
			b = append(b, "48;2;"...)
		} else {
			b = append(b, "38;2;"...)
		}
		b = strconv.AppendInt(b, int64(code>>16&255), 10)
		b = append(b, ';')
		b = strconv.AppendInt(b, int64(code>>8&255), 10)
		b = append(b, ';')
		return strconv.AppendInt(b, int64(code&255), 10)
	case cm256:
		if bg {
			b = append(b, "48;5;"...)
		} else {
			b = append(b, "38;5;"...)
		}
		return strconv.AppendInt(b, int64(code), 10)
	default:
		i := int64(code)
		if bg {
			return strconv.AppendInt(b, 40+i%8, 10) // los fondos brillantes no son confiables
		}
		if i == 0 { // nunca texto negro sobre negro
			i = 8
		}
		if i >= 8 {
			return strconv.AppendInt(b, 90+i-8, 10)
		}
		return strconv.AppendInt(b, 30+i, 10)
	}
}

// cuadro de celdas
type cellK struct {
	ch     rune
	fg, bg int32
}

// frame es una pantalla ya convertida a códigos de color, lista para comparar.
type frame struct {
	w, h int
	c    []cellK
}

func (cv *canvas) toFrame(f *frame) *frame {
	n := cv.w * cv.h
	if f == nil || cap(f.c) < n {
		f = &frame{c: make([]cellK, n)}
	}
	f.w, f.h, f.c = cv.w, cv.h, f.c[:n]
	i := 0
	for y := 0; y < cv.h; y++ {
		row := cv.c[y]
		for x := 0; x < cv.w; x++ {
			t := row[x]
			ch := t.Ch
			if ch == 0 {
				ch = ' '
			}
			f.c[i] = cellK{ch, cv.mode.code(t.Fg), cv.mode.code(t.Bg)}
			i++
		}
	}
	return f
}

// salida
type lockedW struct {
	mu sync.Mutex
	w  io.Writer
	n  atomic.Int64
}

func (l *lockedW) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.n.Add(int64(len(p)))
	return l.w.Write(p)
}

// rawRenderer manda solo las celdas que cambiaron (mover cursor + SGR mínimo) desde
// su propia goroutine, así una conexión lenta no bloquea la entrada. Si un cuadro
// no alcanza a salir se descarta y gana el más nuevo. Máximo ~20 fps.
type rawRenderer struct {
	out    *lockedW
	mode   colorMode
	ch     chan *frame
	full   atomic.Bool
	done   chan struct{}
	last   *frame
	spare  chan *frame
	buf    []byte
	minGap time.Duration
}

func newRawRenderer(out *lockedW, mode colorMode) *rawRenderer {
	r := &rawRenderer{out: out, mode: mode, ch: make(chan *frame, 1), spare: make(chan *frame, 2), done: make(chan struct{}), minGap: 50 * time.Millisecond}
	r.full.Store(true)
	go r.loop()
	return r
}

func (r *rawRenderer) stop() {
	select {
	case <-r.done:
	default:
		close(r.done)
	}
}

// getFrame devuelve un cuadro reciclado.
func (r *rawRenderer) getFrame() *frame {
	select {
	case f := <-r.spare:
		return f
	default:
		return nil
	}
}

func (r *rawRenderer) recycle(f *frame) {
	select {
	case r.spare <- f:
	default:
	}
}

func (r *rawRenderer) submit(f *frame) {
	for {
		select {
		case r.ch <- f:
			return
		default:
		}
		select {
		case old := <-r.ch: // se tira el cuadro viejo
			r.recycle(old)
		default:
		}
	}
}

func (r *rawRenderer) loop() {
	var lastWrite time.Time
	for {
		var f *frame
		select {
		case <-r.done:
			return
		case f = <-r.ch:
		}
		if d := r.minGap - time.Since(lastWrite); d > 0 {
			select {
			case <-r.done:
				return
			case <-time.After(d):
			}
			select { // pudo llegar un cuadro más nuevo
			case nf := <-r.ch:
				r.recycle(f)
				f = nf
			default:
			}
		}
		r.buf = r.encode(r.buf[:0], f)
		if len(r.buf) > 0 {
			if _, err := r.out.Write(r.buf); err != nil {
				return
			}
			lastWrite = time.Now()
		}
		if r.last != nil {
			r.recycle(r.last)
		}
		r.last = f
	}
}

func (r *rawRenderer) encode(b []byte, f *frame) []byte {
	full := r.full.Swap(false) || r.last == nil || r.last.w != f.w || r.last.h != f.h
	curFg, curBg := int32(-2), int32(-2)
	cx, cy := -1, -1
	if full {
		b = append(b, "\x1b[0m\x1b[H"...)
		cx, cy, curFg, curBg = 0, 0, -1, -1
	}
	for y := 0; y < f.h; y++ {
		row := f.c[y*f.w : (y+1)*f.w]
		var prev []cellK
		if !full {
			prev = r.last.c[y*f.w : (y+1)*f.w]
		}
		for x := 0; x < f.w; x++ {
			c := row[x]
			if !full && prev[x] == c {
				continue
			}
			if cy != y || cx != x {
				// hueco corto en la misma fila: sale más barato reescribir las celdas iguales
				if cy == y && x > cx && x-cx <= 3 && !full {
					for gx := cx; gx < x; gx++ {
						b = r.cell(b, row[gx], &curFg, &curBg)
					}
				} else {
					b = append(b, "\x1b["...)
					b = strconv.AppendInt(b, int64(y+1), 10)
					b = append(b, ';')
					b = strconv.AppendInt(b, int64(x+1), 10)
					b = append(b, 'H')
				}
			}
			b = r.cell(b, c, &curFg, &curBg)
			cx, cy = x+1, y
		}
		if full && y < f.h-1 {
			cx = -1 // mover el cursor a mano en cada fila (evita líos con autowrap)
		}
	}
	if len(b) > 0 {
		b = append(b, "\x1b[0m"...)
	}
	return b
}

func (r *rawRenderer) cell(b []byte, c cellK, curFg, curBg *int32) []byte {
	if c.fg != *curFg || c.bg != *curBg {
		b = append(b, "\x1b["...)
		if c.fg != *curFg {
			b = r.mode.appendSGR(b, c.fg, false)
			if c.bg != *curBg {
				b = append(b, ';')
			}
		}
		if c.bg != *curBg {
			b = r.mode.appendSGR(b, c.bg, true)
		}
		b = append(b, 'm')
		*curFg, *curBg = c.fg, c.bg
	}
	if c.ch < 0x80 {
		return append(b, byte(c.ch))
	}
	return utf8Append(b, c.ch)
}

func utf8Append(b []byte, r rune) []byte {
	var tmp [4]byte
	n := encodeRune(tmp[:], r)
	return append(b, tmp[:n]...)
}

// canvas.String rápido (título, menús, tests)
func (cv *canvas) fastString() string {
	var bb bytes.Buffer
	b := make([]byte, 0, cv.w*cv.h*4)
	for y := 0; y < cv.h; y++ {
		curFg, curBg := int32(-2), int32(-2)
		for x := 0; x < cv.w; x++ {
			t := cv.c[y][x]
			fg, bg := cv.mode.code(t.Fg), cv.mode.code(t.Bg)
			if fg != curFg || bg != curBg {
				b = append(b, "\x1b["...)
				if fg != curFg {
					b = cv.mode.appendSGR(b, fg, false)
					if bg != curBg {
						b = append(b, ';')
					}
				}
				if bg != curBg {
					b = cv.mode.appendSGR(b, bg, true)
				}
				b = append(b, 'm')
				curFg, curBg = fg, bg
			}
			ch := t.Ch
			if ch == 0 {
				ch = ' '
			}
			if ch < 0x80 {
				b = append(b, byte(ch))
			} else {
				b = utf8Append(b, ch)
			}
		}
		b = append(b, "\x1b[0m"...)
		if y < cv.h-1 {
			b = append(b, '\n')
		}
	}
	bb.Write(b)
	return bb.String()
}

func encodeRune(p []byte, r rune) int {
	switch {
	case r < 0x800:
		p[0] = byte(0xC0 | r>>6)
		p[1] = byte(0x80 | r&0x3F)
		return 2
	case r < 0x10000:
		p[0] = byte(0xE0 | r>>12)
		p[1] = byte(0x80 | (r>>6)&0x3F)
		p[2] = byte(0x80 | r&0x3F)
		return 3
	default:
		p[0] = byte(0xF0 | r>>18)
		p[1] = byte(0x80 | (r>>12)&0x3F)
		p[2] = byte(0x80 | (r>>6)&0x3F)
		p[3] = byte(0x80 | r&0x3F)
		return 4
	}
}
