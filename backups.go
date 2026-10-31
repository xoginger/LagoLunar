// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Respaldos diarios de accounts.json y bans.json en DATA_DIR/backups.
// Se hace uno al arrancar y otro cada día; se guardan los últimos 14.
const keepBackups = 14

var backupFiles = []string{"accounts.json", "bans.json"}

// backupNow copia los archivos con la fecha del día en el nombre, por ejemplo
// backups/accounts-2026-10-31.json. Si ya existe el de hoy se reemplaza.
func backupNow(dir string, now time.Time) error {
	bd := filepath.Join(dir, "backups")
	if err := os.MkdirAll(bd, 0700); err != nil {
		return err
	}
	day := now.Format("2006-01-02")
	for _, f := range backupFiles {
		b, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			continue // todavía no existe, nada que respaldar
		}
		base := strings.TrimSuffix(f, ".json")
		dst := filepath.Join(bd, base+"-"+day+".json")
		if err := os.WriteFile(dst+".tmp", b, 0600); err != nil {
			return err
		}
		if err := os.Rename(dst+".tmp", dst); err != nil {
			return err
		}
		pruneBackups(bd, base)
	}
	return nil
}

// pruneBackups borra los respaldos más viejos de un archivo y deja keepBackups.
// Los nombres llevan la fecha en formato ISO, así que ordenar el texto basta.
func pruneBackups(bd, base string) {
	old, _ := filepath.Glob(filepath.Join(bd, base+"-????-??-??.json"))
	sort.Strings(old)
	for len(old) > keepBackups {
		os.Remove(old[0])
		old = old[1:]
	}
}

// startBackups respalda al arrancar y luego revisa cada hora si ya cambió el día.
func startBackups() {
	dir := dataDir()
	last := time.Now()
	if err := backupNow(dir, last); err != nil {
		log.Warn("respaldo", "err", err)
	}
	go func() {
		for range time.Tick(time.Hour) {
			now := time.Now()
			if now.Format("2006-01-02") == last.Format("2006-01-02") {
				continue
			}
			last = now
			if err := backupNow(dir, now); err != nil {
				log.Warn("respaldo", "err", err)
			}
		}
	}()
}
