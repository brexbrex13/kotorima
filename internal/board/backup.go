package board

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Backup serialises the database and attachments under the mutation lock, then
// atomically renames the completed ZIP. Originals are never replaced.
func (s *Store) Backup() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.Dir, "backups")
	if e := os.MkdirAll(dir, 0755); e != nil {
		return "", e
	}
	f, e := os.CreateTemp(dir, ".backup-*.tmp")
	if e != nil {
		return "", e
	}
	temp := f.Name()
	defer os.Remove(temp)
	z := zip.NewWriter(f)
	dump := map[string][]json.RawMessage{}
	for _, table := range []string{"tasks", "categories", "series", "notifications", "metadata"} {
		column := "data"
		if table == "metadata" {
			column = "json_object('key',key,'value',value)"
		}
		rows, e := s.db.Query("SELECT " + column + " FROM " + table)
		if e != nil {
			z.Close()
			f.Close()
			return "", e
		}
		values := []json.RawMessage{}
		for rows.Next() {
			var b string
			if e = rows.Scan(&b); e != nil {
				rows.Close()
				z.Close()
				f.Close()
				return "", e
			}
			values = append(values, json.RawMessage(b))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			z.Close()
			f.Close()
			return "", e
		}
		dump[table] = values
	}
	b, e := json.MarshalIndent(map[string]any{"format": "memotodo-board-v3", "version": 1, "tables": dump}, "", "  ")
	if e != nil {
		z.Close()
		f.Close()
		return "", e
	}
	entry, e := z.Create("board.json")
	if e == nil {
		_, e = entry.Write(b)
	}
	if e == nil {
		e = filepath.WalkDir(filepath.Join(s.Dir, "images"), func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("画像フォルダのリンクはバックアップできません")
			}
			file, e := os.Open(path)
			if e != nil {
				return e
			}
			defer file.Close()
			relative, e := filepath.Rel(s.Dir, path)
			if e != nil {
				return e
			}
			entry, e := z.Create(filepath.ToSlash(relative))
			if e != nil {
				return e
			}
			_, e = io.Copy(entry, file)
			return e
		})
	}
	ze := z.Close()
	fe := f.Close()
	if e != nil {
		return "", e
	}
	if ze != nil {
		return "", ze
	}
	if fe != nil {
		return "", fe
	}
	target := filepath.Join(dir, "Kotorima-"+s.now().Format("20060102-150405.000000000")+".zip")
	if e = os.Rename(temp, target); e != nil {
		return "", e
	}
	return target, nil
}
func (s *Store) DailyBackup() error {
	dir := filepath.Join(s.Dir, "backups")
	today := "daily-" + Date(s.now()) + ".zip"
	if _, e := os.Stat(filepath.Join(dir, today)); e == nil {
		return nil
	}
	path, e := s.Backup()
	if e != nil {
		return e
	}
	if e = os.Rename(path, filepath.Join(dir, today)); e != nil {
		return e
	}
	entries, _ := os.ReadDir(dir)
	days := []string{}
	for _, v := range entries {
		if strings.HasPrefix(v.Name(), "daily-") && strings.HasSuffix(v.Name(), ".zip") {
			days = append(days, v.Name())
		}
	}
	sort.Strings(days)
	for len(days) > 10 {
		if e = os.Remove(filepath.Join(dir, days[0])); e != nil {
			return e
		}
		days = days[1:]
	}
	return nil
}
