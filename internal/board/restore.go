package board

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Restore accepts only our explicit export format. It preserves all existing
// images, uses new filenames for conflicting attachments, and commits all rows
// together. A failed restore may leave orphan images, never a half restored DB.
func (s *Store) Restore(data []byte) error {
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return e
	}
	return s.restoreZIP(z)
}

// Desktop restoration opens the ZIP directly; attachments are streamed to disk,
// never copied through base64 or collected into one in-memory image map.
func (s *Store) RestoreFile(path string) error {
	z, e := zip.OpenReader(path)
	if e != nil {
		return e
	}
	defer z.Close()
	return s.restoreZIP(&z.Reader)
}
func (s *Store) restoreZIP(z *zip.Reader) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stage, e := os.MkdirTemp(s.Dir, ".restore-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	var dump struct {
		Format  string                       `json:"format"`
		Version int                          `json:"version"`
		Tables  map[string][]json.RawMessage `json:"tables"`
	}
	files := map[string]bool{}
	seenNames := map[string]bool{}
	for _, f := range z.File {
		if f.Name != "board.json" && (!strings.HasPrefix(f.Name, "images/") || strings.TrimPrefix(f.Name, "images/") == "" || strings.TrimPrefix(f.Name, "images/") == "." || strings.TrimPrefix(f.Name, "images/") == ".." || filepath.Base(f.Name) != strings.TrimPrefix(f.Name, "images/") || strings.ContainsAny(f.Name, "\\:") || f.Mode()&os.ModeSymlink != 0) {
			return errors.New("バックアップ内のパスが不正です")
		}
		if seenNames[strings.ToLower(f.Name)] {
			return errors.New("バックアップ内に重複ファイルがあります")
		}
		files[f.Name] = true
		seenNames[strings.ToLower(f.Name)] = true
		r, e := f.Open()
		if e != nil {
			return e
		}
		if f.Name == "board.json" {
			decoder := json.NewDecoder(r)
			e = decoder.Decode(&dump)
			if e == nil {
				var extra any
				if tail := decoder.Decode(&extra); tail != io.EOF {
					if tail == nil {
						e = errors.New("バックアップの内容が不正です")
					} else {
						e = tail
					}
				}
			} // Verify the ZIP checksum as well.
		} else {
			out, err := os.Create(filepath.Join(stage, strings.TrimPrefix(f.Name, "images/")))
			if err != nil {
				r.Close()
				return err
			}
			_, e = io.Copy(out, r)
			closeErr := out.Close()
			if e == nil {
				e = closeErr
			}
		}
		r.Close()
		if e != nil {
			return e
		}
	}
	if !files["board.json"] || dump.Format != "memotodo-board-v3" || dump.Version != 1 {
		return errors.New("KotorimaまたはMemoTodo v3のバックアップを選んでください")
	}
	for _, table := range []string{"tasks", "categories", "series", "notifications", "metadata"} {
		if _, ok := dump.Tables[table]; !ok {
			return errors.New("バックアップ内のテーブルが不足しています")
		}
	}
	remap := map[string]string{}
	for path := range files {
		if path == "board.json" {
			continue
		}
		original := strings.TrimPrefix(path, "images/")
		name := original
		target := filepath.Join(s.Dir, "images", name)
		if _, err := os.Stat(target); err == nil {
			same, err := sameFile(target, filepath.Join(stage, original))
			if err != nil {
				return err
			}
			if same {
				continue
			}
			name = uuid.NewString() + filepath.Ext(name)
			remap[original] = name
			target = filepath.Join(s.Dir, "images", name)
		} else if !os.IsNotExist(err) {
			return err
		}
		if e = os.Rename(filepath.Join(stage, original), target); e != nil {
			return e
		}
	}
	rewrite := func(memo string) string {
		for old, name := range remap {
			memo = strings.ReplaceAll(memo, "/images/"+old, "/images/"+name)
		}
		return memo
	}
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, table := range []string{"tasks", "categories", "series", "notifications", "metadata"} {
		if _, e = tx.Exec("DELETE FROM " + table); e != nil {
			return e
		}
	}
	for _, b := range dump.Tables["metadata"] {
		var v struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		}
		switch v.Key {
		case "settings":
			var settings Settings
			if e = json.Unmarshal([]byte(v.Value), &settings); e != nil {
				return e
			}
		case "category_model":
			if v.Value != "1" {
				return errors.New("カテゴリ形式が不正です")
			}
		case "window_size":
			var size [2]int
			if e = json.Unmarshal([]byte(v.Value), &size); e != nil {
				return e
			}
		case "summary_cursor":
			if !strings.HasPrefix(v.Value, "summary:") {
				return errors.New("定時通知の記録が不正です")
			}
			if _, e = time.Parse("2006-01-02:15:04", strings.TrimPrefix(v.Value, "summary:")); e != nil {
				return errors.New("定時通知の記録が不正です")
			}
		default:
			return fmt.Errorf("未知の設定: %s", v.Key)
		}
		if _, e = tx.Exec("INSERT INTO metadata(key,value) VALUES(?,?)", v.Key, v.Value); e != nil {
			return e
		}
	}
	restoredSettings, e := s.settings(tx)
	if e != nil {
		return e
	}
	for _, b := range dump.Tables["categories"] {
		var c Category
		if e = json.Unmarshal(b, &c); e != nil {
			return e
		}
		if c.ID < 1 || c.Name == "" || !validColor(c.Color) {
			return errors.New("カテゴリが不正です")
		}
		encoded, _ := json.Marshal(c)
		if _, e = tx.Exec("INSERT INTO categories(id,data) VALUES(?,?)", c.ID, string(encoded)); e != nil {
			return e
		}
	}
	for _, b := range dump.Tables["series"] {
		var v Series
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		}
		if v.ID < 1 {
			return errors.New("定期設定IDが不正です")
		}
		if v.ShowDays == -1 {
			v.ShowDays = restoredSettings.SeriesShowDays
			v.Version++
		}
		if e = validateSeries(v); e != nil {
			return e
		}
		if _, e = ParseTime(v.NextDue, s.now().Location()); e != nil {
			return e
		}
		v.Memo = rewrite(v.Memo)
		encoded, _ := json.Marshal(v)
		if _, e = tx.Exec("INSERT INTO series(id,data) VALUES(?,?)", v.ID, string(encoded)); e != nil {
			return e
		}
	}
	for _, b := range dump.Tables["tasks"] {
		var v Task
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		}
		if v.ID < 1 {
			return errors.New("付箋IDが不正です")
		}
		if e = validateTask(v); e != nil {
			return e
		}
		v.Memo = rewrite(v.Memo)
		encoded, _ := json.Marshal(v)
		if _, e = tx.Exec("INSERT INTO tasks(id,series_id,occurrence,data) VALUES(?,?,?,?)", v.ID, v.SeriesID, v.Occurrence, string(encoded)); e != nil {
			return e
		}
	}
	for _, b := range dump.Tables["notifications"] {
		var n Notification
		if e = json.Unmarshal(b, &n); e != nil {
			return e
		}
		if n.ID < 1 || n.Key == "" {
			return errors.New("通知が不正です")
		}
		if _, e = tx.Exec("INSERT INTO notifications(id,event_key,data) VALUES(?,?,?)", n.ID, n.Key, string(b)); e != nil {
			return e
		}
	}
	if e = normalizeCategories(tx); e != nil {
		return e
	}
	if e = s.freezeInheritedLead(tx); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.undo = nil
	return nil
}

func sameFile(a, b string) (bool, error) {
	hash := func(path string) ([]byte, error) {
		f, e := os.Open(path)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		h := sha256.New()
		if _, e = io.Copy(h, f); e != nil {
			return nil, e
		}
		return h.Sum(nil), nil
	}
	x, e := hash(a)
	if e != nil {
		return false, e
	}
	y, e := hash(b)
	return bytes.Equal(x, y), e
}
