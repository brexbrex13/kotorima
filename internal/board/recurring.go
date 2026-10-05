package board

import (
	"errors"
	"strings"
	"time"
)

func monthEnd(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }
func nextDue(v Series, after time.Time) time.Time {
	switch v.Period {
	case "daily":
		return after.AddDate(0, 0, v.Interval)
	case "weekly":
		anchor, _ := ParseTime(v.FirstDue, after.Location())
		anchor = anchor.AddDate(0, 0, -(int(anchor.Weekday())+6)%7)
		for d := after.AddDate(0, 0, 1); ; d = d.AddDate(0, 0, 1) {
			days := int(time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).Sub(time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, time.UTC)).Hours() / 24)
			if (days/7)%v.Interval == 0 {
				for _, wd := range v.Weekdays {
					if int(d.Weekday()) == wd {
						return d
					}
				}
			}
		}
	case "monthly":
		y, m := after.Year(), after.Month()+time.Month(v.Interval)
		day := v.MonthDay
		if day == 0 || day > monthEnd(y, m) {
			day = monthEnd(y, m)
		}
		return time.Date(y, m, day, 0, 0, 0, 0, after.Location())
	case "yearly":
		y := after.Year() + v.Interval
		m := time.Month(v.Month)
		day := v.MonthDay
		if day == 0 || day > monthEnd(y, m) {
			day = monthEnd(y, m)
		}
		return time.Date(y, m, day, 0, 0, 0, 0, after.Location())
	}
	return after
}
func validateSeries(v Series) error {
	if strings.TrimSpace(v.Title) == "" || len(v.Title) > 50000 || len(v.Memo) > 8000000 {
		return errors.New("定期付箋の内容を入力してください")
	}
	if v.Interval < 1 || v.Interval > 100 {
		return errors.New("周期間隔は1〜100です")
	}
	if v.Period != "daily" && v.Period != "weekly" && v.Period != "monthly" && v.Period != "yearly" {
		return errors.New("周期が不正です")
	}
	if _, e := time.Parse("2006-01-02", v.FirstDue); e != nil {
		return errors.New("初回期限が不正です")
	}
	if v.EndDate != "" {
		if _, e := time.Parse("2006-01-02", v.EndDate); e != nil || v.EndDate < v.FirstDue {
			return errors.New("終了日が不正です")
		}
	}
	if v.ShowDays < 0 || v.ShowDays > 366 || v.NearDays < 0 || v.NearDays > 366 || v.NotifyDays < 0 || v.NotifyDays > 366 || v.NotifyHours < 0 || v.NotifyHours > 8784 {
		return errors.New("日数・時間が範囲外です")
	}
	if v.NotifyMode != "off" && v.NotifyMode != "days" && v.NotifyMode != "hours" {
		return errors.New("通知方式が不正です")
	}
	if v.Period == "weekly" {
		if len(v.Weekdays) == 0 {
			return errors.New("曜日を選んでください")
		}
		for _, d := range v.Weekdays {
			if d < 0 || d > 6 {
				return errors.New("曜日が不正です")
			}
		}
	}
	if v.MonthDay < 0 || v.MonthDay > 31 || v.Month < 1 || v.Month > 12 {
		return errors.New("月日が不正です")
	}
	if v.DueTime != "" {
		if _, e := time.Parse("15:04", v.DueTime); e != nil {
			return errors.New("期限時刻が不正です")
		}
	}
	if v.NotifyMode == "days" {
		if _, e := time.Parse("15:04", v.NotifyTime); e != nil {
			return errors.New("通知時刻が不正です")
		}
	}
	// A reminder must not precede the moment its sticky is generated.
	lead := v.NotifyDays
	if v.NotifyMode == "hours" {
		lead = (v.NotifyHours + 23) / 24
	}
	if v.NotifyMode != "off" && v.ShowDays < lead {
		return errors.New("付箋の追加時期を通知より前に設定してください")
	}
	return nil
}
func (s *Store) SaveSeries(v Series) (Series, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, beginErr := s.db.Begin()
	if beginErr != nil {
		return v, beginErr
	}
	defer tx.Rollback()
	settings, e := s.settings(tx)
	if e != nil {
		return v, e
	}
	if v.FirstDue == "" {
		v.FirstDue = Date(firstDue(v, s.now()))
	}
	// Day-only deadlines; notifications are independently configured.
	v.DueTime = ""
	v.EndDate = ""
	resolved := v
	if resolved.ShowDays == -1 {
		resolved.ShowDays = settings.SeriesShowDays
	}
	v.ShowDays = resolved.ShowDays
	if e := validateSeries(resolved); e != nil {
		return v, e
	}
	if v.CategoryID == 0 {
		var e error
		v.CategoryID, e = defaultCategory(tx)
		if e != nil {
			return v, e
		}
	}
	if v.CategoryID != 0 {
		if _, e := load[Category](tx, "categories", v.CategoryID); e != nil {
			return v, e
		}
	}
	if v.ID == 0 {
		r, e := tx.Exec("INSERT INTO series(data) VALUES('{}')")
		if e != nil {
			return v, e
		}
		v.ID, _ = r.LastInsertId()
		v.Version = 1
		v.NextDue = v.FirstDue
		v.Deleted = false
	} else {
		old, e := load[Series](tx, "series", v.ID)
		if e != nil {
			return v, e
		}
		if old.Version != v.Version {
			return v, errors.New("定期設定が更新されました。再読込してください")
		}
		v.Version++
		v.NextDue = old.NextDue
		v.Deleted = old.Deleted
		changed := v.Period != old.Period || v.Interval != old.Interval || v.FirstDue != old.FirstDue || v.MonthDay != old.MonthDay || v.Month != old.Month || !equalDays(v.Weekdays, old.Weekdays)
		if changed {
			v.FirstDue = Date(firstDue(v, s.now()))
			v.NextDue = v.FirstDue
			for v.NextDue < Date(s.now()) {
				d, _ := ParseTime(v.NextDue, s.now().Location())
				v.NextDue = Date(nextDue(v, d))
			}
		}
		if !old.Active && v.Active {
			for v.NextDue < Date(s.now()) {
				d, _ := ParseTime(v.NextDue, s.now().Location())
				v.NextDue = Date(nextDue(v, d))
			}
		}
	}
	if e := put(tx, "series", v.ID, v); e != nil {
		return v, e
	}
	return v, tx.Commit()
}
func equalDays(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
func (s *Store) StopSeries(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := load[Series](s.db, "series", id)
	if e != nil {
		return e
	}
	v.Active = false
	v.Deleted = true
	v.Version++
	return put(s.db, "series", id, v)
}
func syncSeries(q queryer, now time.Time) (bool, error) {
	all, e := list[Series](q, "series")
	if e != nil {
		return false, e
	}
	changed := false
	settings, e := (&Store{}).settings(q)
	if e != nil {
		return false, e
	}
	for _, v := range all {
		if !v.Active || v.Deleted {
			continue
		}
		cursor, e := ParseTime(v.NextDue, now.Location())
		if e != nil {
			return false, e
		}
		lead := v.ShowDays
		if lead == -1 {
			lead = settings.SeriesShowDays
		}
		limit := Date(now.AddDate(0, 0, lead))
		count := 0
		for Date(cursor) <= limit && (v.EndDate == "" || Date(cursor) <= v.EndDate) {
			count++
			if count > 100000 {
				return false, errors.New("定期付箋の生成数が多すぎます。開始日を確認してください")
			}
			due := Date(cursor)
			deadline := due
			if v.DueTime != "" {
				deadline += "T" + v.DueTime + ":00"
			}
			t := Task{Version: 1, Title: v.Title, Memo: v.Memo, ShowMemoInNotice: v.ShowMemoInNotice, Status: "pending", Deadline: deadline, CategoryID: v.CategoryID, Important: v.Important, SortOrder: now.UnixNano() + int64(count), CreatedAt: ISO(now), SeriesID: v.ID, Occurrence: due, NearDays: v.NearDays}
			switch v.NotifyMode {
			case "days":
				h, _ := time.Parse("15:04", v.NotifyTime)
				d := cursor.AddDate(0, 0, -v.NotifyDays)
				t.ReminderAt = ISO(time.Date(d.Year(), d.Month(), d.Day(), h.Hour(), h.Minute(), 0, 0, now.Location()))
			case "hours":
				d, _ := Due(t, now.Location())
				t.ReminderAt = ISO(d.Add(-time.Duration(v.NotifyHours) * time.Hour))
			}
			r, e := q.Exec("INSERT OR IGNORE INTO tasks(series_id,occurrence,data) VALUES(?,?,'{}')", v.ID, due)
			if e != nil {
				return false, e
			}
			n, e := r.RowsAffected()
			if e != nil {
				return false, e
			}
			if n > 0 {
				t.ID, e = r.LastInsertId()
				if e != nil {
					return false, e
				}
				if e = put(q, "tasks", t.ID, t); e != nil {
					return false, e
				}
				changed = true
			}
			cursor = nextDue(v, cursor)
		}
		if Date(cursor) != v.NextDue {
			v.NextDue = Date(cursor)
			if e = put(q, "series", v.ID, v); e != nil {
				return false, e
			}
			changed = true
		}
	}
	return changed, nil
}
