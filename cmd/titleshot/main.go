// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

// Titleshot se conecta, manda teclas opcionales y guarda la salida cruda en un archivo.
// uso: titleshot addr usuario teclas salida.ansi
package main

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

func main() {
	addr, user, keys, outf := os.Args[1], os.Args[2], os.Args[3], os.Args[4]
	cfg := &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password("x")}, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		panic(err)
	}
	s, _ := c.NewSession()
	term := os.Getenv("TTERM")
	if term == "" {
		term = "xterm-256color"
	}
	ww, hh := 100, 30
	if os.Getenv("TINY") != "" {
		ww, hh = 0, 0
	}
	s.RequestPty(term, hh, ww, ssh.TerminalModes{})
	var buf bytes.Buffer
	var mu sync.Mutex
	out, _ := s.StdoutPipe()
	in, _ := s.StdinPipe()
	go func() {
		b := make([]byte, 4096)
		for {
			n, e := out.Read(b)
			mu.Lock()
			buf.Write(b[:n])
			mu.Unlock()
			if e != nil {
				return
			}
		}
	}()
	s.Shell()
	time.Sleep(1500 * time.Millisecond)
	for _, k := range keys {
		if k == '~' {
			time.Sleep(time.Second)
			continue
		}
		if k == 'E' {
			in.Write([]byte("\r"))
		} else {
			in.Write([]byte(string(k)))
		}
		time.Sleep(250 * time.Millisecond)
	}
	time.Sleep(700 * time.Millisecond)
	if h := os.Getenv("HOLD"); h != "" {
		d, _ := time.ParseDuration(h)
		time.Sleep(d)
	}
	mu.Lock()
	os.WriteFile(outf, buf.Bytes(), 0644)
	fmt.Println("bytes:", buf.Len())
	mu.Unlock()
	in.Write([]byte{3})
}
