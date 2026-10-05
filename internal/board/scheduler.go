package board

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func addNotification(q queryer, n Notification) (bool, error) {
	r, e := q.Exec("INSERT OR IGNORE INTO notifications(event_key,data) VALUES(?,'{}')", n.Key)
	if e != nil {
		return false, e
	}
	count, e := r.RowsAffected()
	if e != nil || count == 0 {
		return false, e
	}
	n.ID, e = r.LastInsertId()
	if e != nil {
		return false, e
	}
	return true, put(q, "notifications", n.ID, n)
}

// Tick uses wall time, so app downtime, sleep, midnight and clock changes do not
// lose scheduled reminders. Persisted keys make retries and restarts idempotent.
func (s *Store) Tick(now time.Time, startup bool) (bool, error) {
	return s.tick(now, startup, true)
}

// Manual edits can update reminders but must not replay an elapsed summary slot.
func (s *Store) SyncChanges(now time.Time) (bool, error) {
	return s.tick(now, false, false)
}

func (s *Store) tick(now time.Time, startup, summaries bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.db.Begin()
	if e != nil {
		return false, e
	}
	defer tx.Rollback()
	changed, e := syncSeries(tx, now)
	if e != nil {
		return false, e
	}
	settings, e := s.settings(tx)
	if e != nil {
		return false, e
	}
	ts, e := list[Task](tx, "tasks")
	if e != nil {
		return false, e
	}
	pending := map[int64]bool{}
	for _, t := range ts {
		if t.Status != "pending" || t.DeletedAt != "" {
			continue
		}
		pending[t.ID] = true
		if t.ReminderAt == "" || t.NotifiedAt == t.ReminderAt {
			continue
		}
		at, e := ParseTime(t.ReminderAt, now.Location())
		if e != nil {
			return false, e
		}
		if at.After(now) {
			continue
		}
		yes, e := addNotification(tx, Notification{Key: fmt.Sprintf("reminder:%d:%s", t.ID, t.ReminderAt), TaskID: t.ID, Kind: "reminder", Title: t.Title, FiredAt: ISO(now)})
		if e != nil {
			return false, e
		}
		changed = changed || yes
		t.NotifiedAt = t.ReminderAt
		if e = put(tx, "tasks", t.ID, t); e != nil {
			return false, e
		}
	}
	ns, e := list[Notification](tx, "notifications")
	if e != nil {
		return false, e
	}
	for _, n := range ns {
		if !n.Acknowledged && n.TaskID != 0 && !pending[n.TaskID] {
			n.Acknowledged = true
			if e = put(tx, "notifications", n.ID, n); e != nil {
				return false, e
			}
			changed = true
		}
	}
	// Only the latest elapsed summary slot is delivered after sleep; don't replay all.
	slot := ""
	allowed := false
	for _, d := range settings.NotifyWeekdays {
		if int(now.Weekday()) == d {
			allowed = true
		}
	}
	if allowed {
		for _, tm := range settings.NotifyTimes {
			if tm <= now.Format("15:04") && tm > slot {
				slot = tm
			}
		}
	}
	if slot != "" && summaries {
		key := "summary:" + Date(now) + ":" + slot
		var consumed string
		if e := tx.QueryRow("SELECT value FROM metadata WHERE key='summary_cursor'").Scan(&consumed); e != nil && !errors.Is(e, sql.ErrNoRows) {
			return false, e
		}
		if consumed == key {
			return changed, tx.Commit()
		}
		// Consume the slot even when there are no tasks. Adding a deadline later
		// must not turn a previously empty summary into a surprise notification.
		if _, e := tx.Exec("INSERT INTO metadata(key,value) VALUES('summary_cursor',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key); e != nil {
			return false, e
		}
		counts := map[string]int{}
		for _, t := range ts {
			if u := Urgency(t, now, settings); u != "" {
				counts[u]++
			}
		}
		if len(counts) > 0 {
			parts := []string{}
			for _, p := range [][2]string{{"overdue", "期限超過"}, {"today", "今日期限"}, {"near", "期限が近い"}} {
				if n := counts[p[0]]; n > 0 {
					parts = append(parts, fmt.Sprintf("%s %d件", p[1], n))
				}
			}
			// One pending summary window; acknowledge obsolete summary events when updating.
			yes, e := addNotification(tx, Notification{Key: key, Kind: "summary", Title: strings.Join(parts, "・"), FiredAt: ISO(now)})
			if e != nil {
				return false, e
			}
			if yes {
				for _, n := range ns {
					if n.Kind == "summary" && !n.Acknowledged {
						n.Acknowledged = true
						if e = put(tx, "notifications", n.ID, n); e != nil {
							return false, e
						}
					}
				}
				changed = true
			}
		}
	}
	return changed, tx.Commit()
}
func (s *Store) Acknowledge(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := list[Notification](s.db, "notifications")
	if e != nil {
		return e
	}
	for _, n := range all {
		if (id == 0 || id == n.ID) && !n.Acknowledged {
			n.Acknowledged = true
			if e = put(s.db, "notifications", n.ID, n); e != nil {
				return e
			}
		}
	}
	return nil
}
func (s *Store) Snooze(notificationID int64, at string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target, e := ParseTime(at, s.now().Location())
	if e != nil || !target.After(s.now()) {
		return errors.New("未来の通知時刻を指定してください")
	}
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	n, e := load[Notification](tx, "notifications", notificationID)
	if e != nil {
		return e
	}
	if n.TaskID == 0 {
		return errors.New("まとめ通知は確認または通知保留を使用してください")
	}
	t, e := load[Task](tx, "tasks", n.TaskID)
	if e != nil {
		return e
	}
	if t.Status != "pending" || t.DeletedAt != "" {
		return errors.New("付箋は既に完了・削除されています")
	}
	t.ReminderAt = ISO(target)
	t.ReminderMode = ""
	t.ReminderTime = ""
	t.NotifiedAt = ""
	t.Version++
	if e = put(tx, "tasks", t.ID, t); e != nil {
		return e
	}
	n.Acknowledged = true
	if e = put(tx, "notifications", n.ID, n); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) TestNotification() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, e := addNotification(s.db, Notification{Key: fmt.Sprintf("test:%d", s.now().UnixNano()), Kind: "test", Title: "通知テスト — このウィンドウは確認するまで残ります", FiredAt: ISO(s.now())})
	return e
}
