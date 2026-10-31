// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// BenchmarkGameFrame mide un cuadro completo (composición + canvas + ANSI) a 207x60 en 256 colores.
func benchFrame(b *testing.B, others int) {
	setupWorld(nil)
	os.Setenv("SPAWN", "1060,590")
	m := &model{w: 207, h: 60, mode: cm256, user: "B", name: []rune("B")}
	m.join()
	var bots []*model
	for i := 0; i < others; i++ {
		o := &model{w: 207, h: 60, mode: cm256, user: fmt.Sprint(i), name: []rune(fmt.Sprint("bot", i))}
		o.join()
		bots = append(bots, o)
	}
	b.ReportAllocs()
	b.ResetTimer()
	n := 0
	for i := 0; i < b.N; i++ {
		n += len(m.View())
	}
	b.ReportMetric(float64(n)/float64(b.N), "bytes/frame")
	b.StopTimer()
	m.leave()
	for _, o := range bots {
		o.leave()
	}
	_ = time.Now
}

func BenchmarkFrame1(b *testing.B)  { benchFrame(b, 0) }
func BenchmarkFrame10(b *testing.B) { benchFrame(b, 9) }

func BenchmarkSim(b *testing.B) {
	setupWorld(nil)
	os.Setenv("SPAWN", "1060,590")
	m := &model{w: 207, h: 60, mode: cm256, user: "B", name: []rune("B")}
	m.join()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		simStep(time.Now())
		mu.Unlock()
	}
	m.leave()
}

// BenchmarkDiff mide los bytes por cuadro del renderer de diferencias con el jugador quieto y el agua animada.
func BenchmarkDiffIdle(b *testing.B) {
	setupWorld(nil)
	os.Setenv("SPAWN", "1060,590")
	m := &model{w: 207, h: 60, mode: cm256, user: "B", name: []rune("B")}
	m.join()
	r := &rawRenderer{mode: cm256}
	var buf []byte
	total := 0
	start0 := time.Now()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cv := m.gameCanvas(207, 60)
		f := cv.toFrame(nil)
		buf = r.encode(buf[:0], f)
		if i > 0 {
			total += len(buf)
		}
		r.last = f
		time.Sleep(50 * time.Millisecond) // 20 fps
	}
	b.ReportMetric(float64(total)/float64(max(1, b.N-1)), "bytes/frame")
	_ = start0
	m.leave()
}
