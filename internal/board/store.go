package board

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"kotorima/internal/jev"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// All mutations, scheduler ticks and snapshots share one lock. A notification and
// its delivered marker are committed in one transaction; no UI event is the ledger.
type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	Dir  string
	now  func() time.Time
	undo map[string]completionUndo
}

func Open(dir string) (*Store, error) {
	if e := os.MkdirAll(filepath.Join(dir, "images"), 0755); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "board.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, Dir: dir, now: time.Now}
	_, e = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=WAL;
 CREATE TABLE IF NOT EXISTS tasks(id INTEGER PRIMARY KEY AUTOINCREMENT, series_id INTEGER NOT NULL DEFAULT 0, occurrence TEXT NOT NULL DEFAULT '', data TEXT NOT NULL);
 CREATE UNIQUE INDEX IF NOT EXISTS occurrence_key ON tasks(series_id,occurrence) WHERE series_id<>0;
 CREATE TABLE IF NOT EXISTS categories(id INTEGER PRIMARY KEY AUTOINCREMENT,data TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS series(id INTEGER PRIMARY KEY AUTOINCREMENT,data TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS notifications(id INTEGER PRIMARY KEY AUTOINCREMENT,event_key TEXT UNIQUE NOT NULL,data TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS metadata(key TEXT PRIMARY KEY,value TEXT NOT NULL);`)
	if e != nil {
		db.Close()
		return nil, e
	}
	// Freeze old inherited lead times at their current value. Common settings
	// now only seed new rules, never change an existing rule's behaviour.
	e = s.freezeInheritedLead(db)
	if e != nil {
		db.Close()
		return nil, e
	}
	tx, e := db.Begin()
	if e == nil {
		e = normalizeCategories(tx)
		if e == nil {
			e = tx.Commit()
		} else {
			tx.Rollback()
		}
	}
	if e != nil {
		db.Close()
		return nil, e
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) freezeInheritedLead(q queryer) error {
	settings, e := s.settings(q)
	if e != nil {
		return e
	}
	_, e = q.Exec("UPDATE series SET data=json_set(data,'$.show_days',?,'$.version',COALESCE(json_extract(data,'$.version'),0)+1) WHERE json_extract(data,'$.show_days')=-1", settings.SeriesShowDays)
	return e
}

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
}

func list[T any](q queryer, table string) ([]T, error) {
	rows, e := q.Query("SELECT id,data FROM " + table + " ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var id int64
		var data string
		if e = rows.Scan(&id, &data); e != nil {
			return nil, e
		}
		var t T
		if e = json.Unmarshal([]byte(data), &t); e != nil {
			return nil, fmt.Errorf("%s #%d: %w", table, id, e)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func put(q queryer, table string, id int64, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = q.Exec("UPDATE "+table+" SET data=? WHERE id=?", string(b), id)
	return e
}
func load[T any](q queryer, table string, id int64) (T, error) {
	var v T
	var b string
	e := q.QueryRow("SELECT data FROM "+table+" WHERE id=?", id).Scan(&b)
	if e != nil {
		return v, e
	}
	e = json.Unmarshal([]byte(b), &v)
	return v, e
}
func (s *Store) settings(q queryer) (Settings, error) {
	v := Defaults()
	var b string
	e := q.QueryRow("SELECT value FROM metadata WHERE key='settings'").Scan(&b)
	if errors.Is(e, sql.ErrNoRows) {
		return v, nil
	}
	if e != nil {
		return v, e
	}
	e = json.Unmarshal([]byte(b), &v)
	return v, e
}
func (s *Store) Snapshot() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var v Snapshot
	var e error
	if v.Tasks, e = list[Task](s.db, "tasks"); e != nil {
		return v, e
	}
	if v.Categories, e = list[Category](s.db, "categories"); e != nil {
		return v, e
	}
	if v.Series, e = list[Series](s.db, "series"); e != nil {
		return v, e
	}
	if v.Notifications, e = list[Notification](s.db, "notifications"); e != nil {
		return v, e
	}
	v.Settings, e = s.settings(s.db)
	return v, e
}
func validateTask(t Task) error {
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("付箋の内容を入力してください")
	}
	if len(t.Title) > 50000 || len(t.Memo) > 8000000 {
		return errors.New("内容が長すぎます")
	}
	if t.Status != "pending" && t.Status != "done" && t.Status != "skipped" {
		return errors.New("不正な状態です")
	}
	for _, v := range []string{t.Deadline, t.ReminderAt} {
		if v != "" {
			if _, e := ParseTime(v, time.Local); e != nil {
				return errors.New("日時が不正です")
			}
		}
	}
	if t.TodayDate != "" {
		if _, e := time.Parse("2006-01-02", t.TodayDate); e != nil {
			return errors.New("今日やるの日付が不正です")
		}
	}
	return nil
}
func (s *Store) SaveTask(t Task) (Task, error) {
	return s.saveTask(t, false)
}

func (s *Store) SaveSuggestedTask(t Task, automaticCategory bool) (Task, error) {
	return s.saveTask(t, automaticCategory && t.ID == 0)
}

func (s *Store) saveTask(t Task, automaticCategory bool) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, beginErr := s.db.Begin()
	if beginErr != nil {
		return t, beginErr
	}
	defer tx.Rollback()
	t.Title = strings.TrimSpace(t.Title)
	if t.Status == "" {
		t.Status = "pending"
	}
	if e := resolveReminder(&t); e != nil {
		return t, e
	}
	if e := validateTask(t); e != nil {
		return t, e
	}
	if automaticCategory && t.CategoryID != 0 {
		c, e := load[Category](tx, "categories", t.CategoryID)
		if e != nil && e != sql.ErrNoRows {
			return t, e
		}
		if e != nil || c.Dormant {
			t.CategoryID = 0
		}
	}
	if t.CategoryID == 0 {
		var e error
		t.CategoryID, e = defaultCategory(tx)
		if e != nil {
			return t, e
		}
	}
	if t.CategoryID != 0 {
		if _, e := load[Category](tx, "categories", t.CategoryID); e != nil {
			return t, errors.New("カテゴリが見つかりません")
		}
	}
	if t.ID == 0 {
		t.CreatedAt = ISO(s.now())
		t.Version = 1
		t.SeriesID = 0
		t.Occurrence = ""
		t.DeletedAt = ""
		t.DoneAt = ""
		t.NotifiedAt = ""
		t.SortOrder = -s.now().UnixNano()
		r, e := tx.Exec("INSERT INTO tasks(data) VALUES('{}')")
		if e != nil {
			return t, e
		}
		t.ID, e = r.LastInsertId()
		if e != nil {
			return t, e
		}
	} else {
		old, e := load[Task](tx, "tasks", t.ID)
		if e != nil {
			return t, e
		}
		if t.Version != old.Version {
			return t, errors.New("別の画面で更新されました。再読込してから編集してください")
		}
		t.SeriesID = old.SeriesID
		t.Occurrence = old.Occurrence
		t.CreatedAt = old.CreatedAt
		t.DeletedAt = old.DeletedAt
		t.SortOrder = old.SortOrder
		t.TodayDate = old.TodayDate
		t.DoneAt = old.DoneAt
		t.NotifiedAt = old.NotifiedAt
		t.Version++
		if old.ReminderAt != t.ReminderAt {
			t.NotifiedAt = ""
		}
		if old.Status != t.Status {
			if t.Status == "pending" {
				t.DoneAt = ""
				t.ReminderAt = ""
				t.ReminderMode = ""
				t.ReminderTime = ""
				t.NotifiedAt = ""
			} else {
				t.DoneAt = ISO(s.now())
			}
		}
	}
	if e := put(tx, "tasks", t.ID, t); e != nil {
		return t, e
	}
	return t, tx.Commit()
}
func (s *Store) SetState(id int64, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := load[Task](s.db, "tasks", id)
	if e != nil {
		return e
	}
	switch state {
	case "pending", "done", "skipped":
		t.Status = state
		t.DoneAt = ISO(s.now())
		if state == "pending" {
			t.DoneAt = ""
			t.ReminderAt = ""
			t.ReminderMode = ""
			t.ReminderTime = ""
			t.NotifiedAt = ""
		}
	case "trash":
		t.DeletedAt = ISO(s.now())
	case "restore":
		t.DeletedAt = ""
	default:
		return errors.New("不正な操作です")
	}
	t.Version++
	return put(s.db, "tasks", id, t)
}
func (s *Store) Reorder(ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for i, id := range ids {
		t, e := load[Task](tx, "tasks", id)
		if e != nil {
			return e
		}
		t.SortOrder = int64(i)
		t.Version++
		if e = put(tx, "tasks", id, t); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) SaveCategory(c Category) (Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, beginErr := s.db.Begin()
	if beginErr != nil {
		return c, beginErr
	}
	defer tx.Rollback()
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return c, errors.New("カテゴリ名を入力してください")
	}
	if !validColor(c.Color) {
		return c, errors.New("色が不正です")
	}
	if c.TextColor == "" {
		c.TextColor = "#302d25"
	}
	if !validColor(c.TextColor) {
		return c, errors.New("文字色が不正です")
	}
	if c.ID != 0 {
		old, e := load[Category](tx, "categories", c.ID)
		if e != nil {
			return c, e
		}
		c.SortOrder = old.SortOrder
	}
	if c.ID != 0 && c.Dormant {
		first, e := defaultCategory(tx)
		if e != nil {
			return c, e
		}
		if first == c.ID {
			return c, errors.New("先頭のカテゴリは標準追加先のため非表示にできません")
		}
	}
	if c.ID == 0 {
		r, e := tx.Exec("INSERT INTO categories(data) VALUES('{}')")
		if e != nil {
			return c, e
		}
		c.ID, _ = r.LastInsertId()
		c.SortOrder = int(c.ID)
	}
	if e := put(tx, "categories", c.ID, c); e != nil {
		return c, e
	}
	return c, tx.Commit()
}
func validColor(v string) bool {
	if len(v) != 7 || v[0] != '#' {
		return false
	}
	for _, r := range v[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}
func (s *Store) DeleteCategory(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	cs, e := orderedCategories(tx)
	if e != nil {
		return e
	}
	if len(cs) <= 1 {
		return errors.New("カテゴリは最低1個必要です")
	}
	if _, e = load[Category](tx, "categories", id); e != nil {
		return e
	}
	var target Category
	for _, c := range cs {
		if c.ID != id {
			target = c
			break
		}
	}
	target.Dormant = false
	if e = put(tx, "categories", target.ID, target); e != nil {
		return e
	}
	ts, e := list[Task](tx, "tasks")
	if e != nil {
		return e
	}
	for _, t := range ts {
		if t.CategoryID == id {
			t.CategoryID = target.ID
			t.Version++
			if e = put(tx, "tasks", t.ID, t); e != nil {
				return e
			}
		}
	}
	ss, e := list[Series](tx, "series")
	if e != nil {
		return e
	}
	for _, v := range ss {
		if v.CategoryID == id {
			v.CategoryID = target.ID
			v.Version++
			if e = put(tx, "series", v.ID, v); e != nil {
				return e
			}
		}
	}
	if _, e = tx.Exec("DELETE FROM categories WHERE id=?", id); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) ValidateSettings(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return validateSettings(s.db, v)
}
func validateSettings(q queryer, v Settings) error {
	if _, _, e := ParseShortcut(v.QuickShortcut); e != nil {
		return e
	}
	if _, e := time.Parse("15:04", v.ReminderDefaultTime); e != nil {
		return errors.New("通知の既定時刻を指定してください")
	}
	if v.SuggestMinCount < 0 || v.SuggestMinCount > 100 {
		return errors.New("候補の最低登録回数は0〜100で指定してください")
	}
	if v.Theme != "" && v.Theme != "light" && v.Theme != "dark" && v.Theme != "system" {
		return errors.New("配色設定が不正です")
	}
	if v.SeriesShowDays < 0 || v.SeriesShowDays > 366 || v.NearDays < 0 || v.NearDays > 366 || v.FontSize < 12 || v.FontSize > 22 {
		return errors.New("設定値が範囲外です")
	}
	for _, p := range v.CustomColors {
		if strings.TrimSpace(p.Name) == "" || !validColor(p.Background) || !validColor(p.Foreground) {
			return errors.New("カスタム配色が不正です")
		}
	}
	all, e := list[Series](q, "series")
	if e != nil {
		return e
	}
	for _, r := range all {
		if !r.Deleted && r.ShowDays == -1 && r.NotifyMode != "off" && r.NotifyDays > v.SeriesShowDays {
			return errors.New("定期タスクの追加を通知より前に設定してください")
		}
	}
	for _, t := range v.NotifyTimes {
		if _, e := time.Parse("15:04", t); e != nil {
			return errors.New("通知時刻が不正です")
		}
	}
	for _, d := range v.NotifyWeekdays {
		if d < 0 || d > 6 {
			return errors.New("曜日が不正です")
		}
	}
	if v.PauseUntil != "" {
		if _, e := ParseTime(v.PauseUntil, time.Local); e != nil {
			return e
		}
	}
	if v.AITimeoutSeconds < 0 || v.AITimeoutSeconds > 120 {
		return errors.New("AIの待ち時間は0〜120秒で指定してください")
	}
	// Incomplete endpoint details are only rejected once smart add is on.
	if e := jev.ValidateEndpoint(v.JevEndpoint()); e != nil {
		switch v.JevProvider {
		case "", "typesafe", "vercel", "cloudflare", "custom", "openai", "local":
			if v.SmartAdd {
				return e
			}
		default:
			return e
		}
	}
	return nil
}
func (s *Store) SaveSettings(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := validateSettings(s.db, v); e != nil {
		return e
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO metadata(key,value) VALUES('settings',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(b))
	return e
}
