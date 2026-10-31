// Lago Lunar - MMORPG ASCII por SSH
// Copyright (c) 2026 Xocotzin Granados
// Licencia MIT, ver LICENSE.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupsRotate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "accounts.json"), []byte(`{}`), 0600)
	d := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	for i := 0; i < 20; i++ {
		if err := backupNow(dir, d.AddDate(0, 0, i)); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := filepath.Glob(filepath.Join(dir, "backups", "accounts-*.json"))
	if len(got) != keepBackups || !strings.HasSuffix(got[len(got)-1], "2026-01-20.json") {
		t.Fatalf("respaldos: %v", got)
	}
}
