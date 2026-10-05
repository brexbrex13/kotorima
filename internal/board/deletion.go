package board

import (
	"fmt"
	"html"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Moving to trash keeps the memo and its images available for restoration.
func (s *Store) TrashCompleted() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	tasks, e := list[Task](tx, "tasks")
	if e != nil {
		return 0, e
	}
	n := 0
	for _, t := range tasks {
		if t.Status != "done" || t.DeletedAt != "" {
			continue
		}
		t.DeletedAt = ISO(s.now())
		t.Version++
		if e = put(tx, "tasks", t.ID, t); e != nil {
			return 0, e
		}
		n++
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return n, nil
}

// Only image URLs belonging to this application's image route are eligible.
var imageSrc = regexp.MustCompile(`(?i)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)

func internalImages(memo string) map[string]bool {
	out := map[string]bool{}
	for _, match := range imageSrc.FindAllStringSubmatch(memo, -1) {
		for _, raw := range match[1:] {
			if raw == "" {
				continue
			}
			u, e := url.Parse(html.UnescapeString(raw))
			if e != nil || u.Scheme != "" || u.Host != "" || !strings.HasPrefix(u.Path, "/images/") {
				continue
			}
			name := strings.TrimPrefix(u.Path, "/images/")
			if name == "" || filepath.Base(name) != name || name == "." || name == ".." || strings.ContainsAny(name, "\\:") {
				continue
			}
			out[name] = true
		}
	}
	return out
}

func (s *Store) EmptyTrash() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	tasks, e := list[Task](tx, "tasks")
	if e != nil {
		return 0, e
	}
	series, e := list[Series](tx, "series")
	if e != nil {
		return 0, e
	}
	candidates, retained, deleted := map[string]bool{}, map[string]bool{}, map[int64]bool{}
	for _, t := range tasks {
		images := retained
		if t.DeletedAt != "" {
			images = candidates
			deleted[t.ID] = true
			if _, e = tx.Exec("DELETE FROM tasks WHERE id=?", t.ID); e != nil {
				return 0, e
			}
			if _, e = tx.Exec("DELETE FROM notifications WHERE json_extract(data,'$.task_id')=?", t.ID); e != nil {
				return 0, e
			}
		}
		for name := range internalImages(t.Memo) {
			images[name] = true
		}
	}
	// Keep existing series references, including paused rules.
	for _, r := range series {
		if r.Deleted {
			continue
		}
		for name := range internalImages(r.Memo) {
			retained[name] = true
		}
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	for token, undo := range s.undo {
		if deleted[undo.Before.ID] {
			delete(s.undo, token)
		}
	}
	var failed bool
	for name := range candidates {
		if retained[name] {
			continue
		}
		path := filepath.Join(s.Dir, "images", name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			continue
		}
		// Do not follow links or remove a directory supplied as an image.
		if err != nil {
			failed = true
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
			failed = true
		}
	}
	if failed {
		return len(deleted), fmt.Errorf("タスクは完全削除しましたが、一部の内部画像を削除できませんでした")
	}
	return len(deleted), nil
}
