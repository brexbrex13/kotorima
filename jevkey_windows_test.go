//go:build windows

package main

import (
	"archive/zip"
	"io"
	"kotorima/internal/board"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDPAPIKeyStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	k := newKeyStore(dir)
	if !k.Supported() {
		t.Fatal("unsupported on windows")
	}
	if v, e := k.Load(); e != nil || v != "" {
		t.Fatal(v, e)
	}
	if e := k.Save("tsk-secret-1234"); e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(filepath.Join(dir, "jev.key"))
	if e != nil || strings.Contains(string(raw), "tsk-secret-1234") {
		t.Fatal("key stored in plain text", e)
	}
	if v, e := k.Load(); e != nil || v != "tsk-secret-1234" {
		t.Fatal(v, e)
	}
	if e := k.Clear(); e != nil {
		t.Fatal(e)
	}
	if e := k.Clear(); e != nil {
		t.Fatal("second clear", e)
	}
	if v, _ := k.Load(); v != "" {
		t.Fatal(v)
	}
}

func TestDPAPIKeyStoreTreatsCorruptFileAsMissing(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "jev.key"), []byte("garbage"), 0o600); e != nil {
		t.Fatal(e)
	}
	if v, e := newKeyStore(dir).Load(); e != nil || v != "" {
		t.Fatal(v, e)
	}
}

func TestBackupExcludesJevKey(t *testing.T) {
	s, e := board.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = newKeyStore(s.Dir).Save("tsk-secret-1234"); e != nil {
		t.Fatal(e)
	}
	path, e := s.Backup()
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(path)
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	for _, f := range z.File {
		if strings.Contains(f.Name, "jev") {
			t.Fatal(f.Name)
		}
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		if strings.Contains(string(b), "tsk-secret-1234") {
			t.Fatal("key in backup", f.Name)
		}
	}
}
