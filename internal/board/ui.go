package board

import (
	"encoding/json"
	"errors"
	"time"
)

func (s *Store) ToggleImportant(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := load[Task](s.db, "tasks", id)
	if e != nil {
		return e
	}
	t.Important = !t.Important
	t.Version++
	return put(s.db, "tasks", id, t)
}

// New rules begin with the first matching deadline on/after registration.
// Their durable cursor subsequently accumulates missed cycles during downtime.
func firstDue(v Series, now time.Time) time.Time {
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch v.Period {
	case "weekly":
		for i := 0; i < 7; i++ {
			for _, wd := range v.Weekdays {
				if int(d.Weekday()) == wd {
					return d
				}
			}
			d = d.AddDate(0, 0, 1)
		}
	case "monthly", "yearly":
		y, m := d.Year(), d.Month()
		if v.Period == "yearly" {
			m = time.Month(v.Month)
		}
		day := v.MonthDay
		if day == 0 || day > monthEnd(y, m) {
			day = monthEnd(y, m)
		}
		candidate := time.Date(y, m, day, 0, 0, 0, 0, d.Location())
		if candidate.Before(d) {
			if v.Period == "yearly" {
				y++
			} else {
				m++
			}
			day = v.MonthDay
			if day == 0 || day > monthEnd(y, m) {
				day = monthEnd(y, m)
			}
			candidate = time.Date(y, m, day, 0, 0, 0, 0, d.Location())
		}
		return candidate
	}
	return d
}
func (s *Store) ReorderCategories(ids []int64) error {
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
	if len(ids) != len(cs) {
		return errors.New("すべてのカテゴリを指定してください")
	}
	seen := map[int64]bool{}
	for i, id := range ids {
		if seen[id] {
			return errors.New("カテゴリが重複しています")
		}
		seen[id] = true
		c, e := load[Category](tx, "categories", id)
		if e != nil {
			return e
		}
		c.SortOrder = i
		if i == 0 {
			c.Dormant = false
		}
		if e = put(tx, "categories", id, c); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) MoveTask(id, categoryID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if categoryID == 0 {
		var e error
		categoryID, e = defaultCategory(s.db)
		if e != nil {
			return e
		}
	}
	if categoryID != 0 {
		if _, e := load[Category](s.db, "categories", categoryID); e != nil {
			return e
		}
	}
	t, e := load[Task](s.db, "tasks", id)
	if e != nil {
		return e
	}
	if t.DeletedAt != "" {
		return errors.New("削除済みのタスクは移動できません")
	}
	t.CategoryID = categoryID
	t.SortOrder = s.now().UnixNano()
	t.Version++
	return put(s.db, "tasks", id, t)
}
func (s *Store) WindowSize() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b string
	var v [2]int
	if s.db.QueryRow("SELECT value FROM metadata WHERE key='window_size'").Scan(&b) == nil && json.Unmarshal([]byte(b), &v) == nil && v[0] >= 360 && v[1] >= 420 {
		return v[0], v[1]
	}
	return 960, 700
}
func (s *Store) SaveWindowSize(width, height int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if width < 360 || height < 420 || width > 20000 || height > 20000 {
		return nil
	}
	b, e := json.Marshal([2]int{width, height})
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO metadata(key,value) VALUES('window_size',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", string(b))
	return e
}

type SeriesPreview struct {
	Deadline   string `json:"deadline"`
	AddAt      string `json:"add_at"`
	ReminderAt string `json:"reminder_at"`
}

func (s *Store) PreviewSeries(v Series) (SeriesPreview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := SeriesPreview{}
	settings, e := s.settings(s.db)
	if e != nil {
		return result, e
	}
	if v.Title == "" {
		v.Title = "preview"
	}
	if v.ShowDays == -1 {
		v.ShowDays = settings.SeriesShowDays
	}
	v.DueTime = ""
	v.EndDate = ""
	if v.FirstDue == "" {
		v.FirstDue = Date(firstDue(v, s.now()))
	}
	if e = validateSeries(v); e != nil {
		return result, e
	}
	due := firstDue(v, s.now())
	if v.ID != 0 {
		old, e := load[Series](s.db, "series", v.ID)
		if e != nil {
			return result, e
		}
		if old.Period == v.Period && old.Interval == v.Interval && old.MonthDay == v.MonthDay && old.Month == v.Month && equalDays(old.Weekdays, v.Weekdays) {
			d, e := ParseTime(old.NextDue, s.now().Location())
			if e == nil {
				for d.Before(time.Date(s.now().Year(), s.now().Month(), s.now().Day(), 0, 0, 0, 0, s.now().Location())) {
					d = nextDue(old, d)
				}
				due = d
			}
		}
	}
	result.Deadline = Date(due)
	result.AddAt = Date(due.AddDate(0, 0, -v.ShowDays))
	if v.NotifyMode == "days" {
		t, _ := time.Parse("15:04", v.NotifyTime)
		d := due.AddDate(0, 0, -v.NotifyDays)
		result.ReminderAt = ISO(time.Date(d.Year(), d.Month(), d.Day(), t.Hour(), t.Minute(), 0, 0, d.Location()))
	}
	return result, nil
}
