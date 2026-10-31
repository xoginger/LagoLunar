// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

// Loadtest abre N sesiones de invitado (207x60, xterm-256color) y mide bytes/s por sesión, quieto y caminando.
package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	addr := os.Args[1]
	n, _ := strconv.Atoi(os.Args[2])
	var total, walkers atomic.Int64
	var wg sync.WaitGroup
	phase := atomic.Int32{}
	var idleB, walkB [64]atomic.Int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cfg := &ssh.ClientConfig{User: fmt.Sprintf("bot%d", i), Auth: []ssh.AuthMethod{ssh.Password("x")}, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
			c, err := ssh.Dial("tcp", addr, cfg)
			if err != nil {
				panic(err)
			}
			s, _ := c.NewSession()
			s.RequestPty("xterm-256color", 60, 207, ssh.TerminalModes{})
			out, _ := s.StdoutPipe()
			in, _ := s.StdinPipe()
			go func() {
				b := make([]byte, 65536)
				for {
					k, e := out.Read(b)
					total.Add(int64(k))
					switch phase.Load() {
					case 1:
						idleB[i].Add(int64(k))
					case 2:
						walkB[i].Add(int64(k))
					}
					if e != nil {
						return
					}
				}
			}()
			s.Shell()
			time.Sleep(1500 * time.Millisecond)
			for _, k := range []string{"s", "s", "\r", "\r", "\r"} {
				in.Write([]byte(k))
				time.Sleep(250 * time.Millisecond)
			}
			walkers.Add(1)
			for phase.Load() < 2 {
				time.Sleep(50 * time.Millisecond)
			}
			keys := "dddddddddaaaaaaaaa"
			for phase.Load() == 2 {
				for _, k := range keys {
					in.Write([]byte(string(k)))
					time.Sleep(60 * time.Millisecond)
				}
			}
			in.Write([]byte{3})
		}(i)
	}
	for walkers.Load() < int64(n) {
		time.Sleep(100 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)
	phase.Store(1)
	time.Sleep(5 * time.Second)
	phase.Store(2)
	time.Sleep(5 * time.Second)
	phase.Store(3)
	var si, sw int64
	for i := 0; i < n; i++ {
		si += idleB[i].Load()
		sw += walkB[i].Load()
	}
	fmt.Printf("sessions=%d idle=%.1f KB/s/session walking=%.1f KB/s/session\n", n, float64(si)/5/1024/float64(n), float64(sw)/5/1024/float64(n))
	time.Sleep(300 * time.Millisecond)
	os.Exit(0)
}
