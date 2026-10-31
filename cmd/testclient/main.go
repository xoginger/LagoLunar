// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

// Testclient es un cliente SSH de prueba: abre un PTY, camina hacia el agua y guarda la salida cruda.
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
	user := "test"
	if len(os.Args) > 1 {
		user = os.Args[1]
	}
	cfg := &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.Password("x")}, HostKeyCallback: ssh.InsecureIgnoreHostKey()}
	c, err := ssh.Dial("tcp", "localhost:2222", cfg)
	if err != nil {
		panic(err)
	}
	s, _ := c.NewSession()
	s.RequestPty("xterm-256color", 30, 100, ssh.TerminalModes{})
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
	time.Sleep(time.Second)
	for i := 0; i < 9; i++ {
		in.Write([]byte("s"))
		time.Sleep(120 * time.Millisecond)
	}
	in.Write([]byte("d"))
	time.Sleep(400 * time.Millisecond)
	mu.Lock()
	os.WriteFile("capture.ansi", buf.Bytes(), 0644)
	fmt.Println("bytes:", buf.Len())
	mu.Unlock()
	in.Write([]byte("q"))
	time.Sleep(200 * time.Millisecond)
}
