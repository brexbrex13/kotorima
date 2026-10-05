package board

import (
	"os"
	"testing"
)

func TestRestorePreservesSummaryCursorAndFreezesInheritedLead(t *testing.T) {
	s := setup(t, "2026-10-01T13:01")
	settings := Defaults()
	settings.NotifyTimes = []string{"13:00"}
	settings.SeriesShowDays = 9
	if e := s.SaveSettings(settings); e != nil {
		t.Fatal(e)
	}
	rule, e := s.SaveSeries(weekly())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("UPDATE series SET data=json_set(data,'$.show_days',-1) WHERE id=?", rule.ID); e != nil {
		t.Fatal(e)
	}
	tick(t, s, "2026-10-01T13:01")
	path, e := s.Backup()
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Restore(data); e != nil {
		t.Fatal(e)
	}
	if snap(t, s).Series[0].ShowDays != 9 {
		t.Fatal("restored legacy lead not frozen")
	}
	var cursor string
	if e = s.db.QueryRow("SELECT value FROM metadata WHERE key='summary_cursor'").Scan(&cursor); e != nil || cursor != "summary:2026-10-01:13:00" {
		t.Fatal(cursor, e)
	}
	before := len(snap(t, s).Notifications)
	tick(t, s, "2026-10-01T13:02")
	if len(snap(t, s).Notifications) != before {
		t.Fatal("restored summary replayed")
	}
}

func TestManualDeadlineDoesNotReplayEmptySummarySlot(t *testing.T) {
	s := setup(t, "2026-10-01T13:01")
	settings := Defaults()
	settings.NotifyTimes = []string{"13:00", "17:00"}
	if e := s.SaveSettings(settings); e != nil {
		t.Fatal(e)
	}
	tick(t, s, "2026-10-01T13:01") // empty slot must still be consumed
	task, e := s.SaveTask(Task{Title: "manually added", Deadline: "2026-10-01"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SyncChanges(at("2026-10-01T13:02")); e != nil {
		t.Fatal(e)
	}
	tick(t, s, "2026-10-01T13:02")
	if len(snap(t, s).Notifications) != 0 {
		t.Fatal("manual deadline replayed a summary")
	}
	task.Deadline = "2026-09-30"
	if _, e = s.SaveTask(task); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SyncChanges(at("2026-10-01T13:03")); e != nil {
		t.Fatal(e)
	}
	if len(snap(t, s).Notifications) != 0 {
		t.Fatal("manual edit produced notification")
	}
	tick(t, s, "2026-10-01T17:00")
	if len(snap(t, s).Notifications) != 1 {
		t.Fatal("next scheduled summary missing")
	}
}

func TestImportantTogglePreservesMemoAndReminder(t *testing.T) {
	s := setup(t, "2026-10-01T10:00")
	task, e := s.SaveTask(Task{Title: "task", Memo: "rich memo", ReminderAt: "2026-10-02T10:00"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ToggleImportant(task.ID); e != nil {
		t.Fatal(e)
	}
	updated := snap(t, s).Tasks[0]
	if !updated.Important || updated.Memo != task.Memo || updated.ReminderAt != task.ReminderAt || updated.Version != task.Version+1 {
		t.Fatal(updated)
	}
	if e = s.SaveWindowSize(360, 500); e != nil {
		t.Fatal(e)
	}
	w, h := s.WindowSize()
	if w != 360 || h != 500 {
		t.Fatal(w, h)
	}
}

func TestLegacyInheritedLeadFrozenOnOpen(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	rule := weekly()
	rule.ShowDays = 7
	v, e := s.SaveSeries(rule)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("UPDATE series SET data=json_set(data,'$.show_days',-1) WHERE id=?", v.ID); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if snap(t, s).Series[0].ShowDays != 7 {
		t.Fatal("legacy rule lost its lead")
	}
	settings := snap(t, s).Settings
	settings.SeriesShowDays = 14
	if e = s.SaveSettings(settings); e != nil {
		t.Fatal(e)
	}
	if snap(t, s).Series[0].ShowDays != 7 {
		t.Fatal("existing lead followed new default")
	}
}
