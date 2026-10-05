package board

import (
	"crypto/rand"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTodaySelectionPersistsAndDoesNotChangeDeadline(t *testing.T) {
	s := setup(t, "2026-10-01T10:00")
	task, e := s.SaveTask(Task{Title: "来週締切", Deadline: "2026-10-08", Status: "pending"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ToggleToday(task.ID); e != nil {
		t.Fatal(e)
	}
	v := snap(t, s).Tasks[0]
	if v.TodayDate != "2026-10-01" || v.Deadline != task.Deadline {
		t.Fatal(v)
	}
	s.now = func() time.Time { v, _ := ParseTime("2026-10-02T10:00", time.Local); return v }
	if v.TodayDate == Date(s.now()) {
		t.Fatal("yesterday's selection leaked into today")
	}
	if e = s.ToggleToday(task.ID); e != nil {
		t.Fatal(e)
	}
	if snap(t, s).Tasks[0].TodayDate != "2026-10-02" {
		t.Fatal("today not selected")
	}
	if e = s.ToggleToday(task.ID); e != nil {
		t.Fatal(e)
	}
	if snap(t, s).Tasks[0].TodayDate != "" {
		t.Fatal("toggle did not remove selection")
	}
}
func TestDeadlineLinkedReminderAndSnooze(t *testing.T) {
	s := setup(t, "2026-10-01T08:00")
	task, e := s.SaveTask(Task{Title: "提出", Status: "pending", Deadline: "2026-10-02", ReminderMode: "deadline", ReminderTime: "09:30"})
	if e != nil {
		t.Fatal(e)
	}
	if task.ReminderAt != "2026-10-02T09:30:00" {
		t.Fatal(task)
	}
	task.Deadline = "2026-10-03"
	task, e = s.SaveTask(task)
	if e != nil {
		t.Fatal(e)
	}
	if task.ReminderAt != "2026-10-03T09:30:00" {
		t.Fatal(task)
	}
	tick(t, s, "2026-10-03T09:31")
	v := snap(t, s)
	if len(v.Notifications) != 1 {
		t.Fatal(v.Notifications)
	}
	if e = s.Snooze(v.Notifications[0].ID, "2026-10-03T10:00"); e != nil {
		t.Fatal(e)
	}
	task = snap(t, s).Tasks[0]
	task.Memo = "あとで確認"
	task, e = s.SaveTask(task)
	if e != nil {
		t.Fatal(e)
	}
	if task.ReminderMode != "" || task.ReminderAt != "2026-10-03T10:00:00" {
		t.Fatal("editing reset snooze", task)
	}
	task.ReminderMode = "deadline"
	task.Deadline = ""
	if _, e = s.SaveTask(task); e == nil {
		t.Fatal("linked reminder without date accepted")
	}
}
func TestUndoCompletionRestoresReminderWithoutRedelivery(t *testing.T) {
	s := setup(t, "2026-10-01T09:01")
	task, e := s.SaveTask(Task{Title: "確認", Status: "pending", ReminderAt: "2026-10-01T09:00"})
	if e != nil {
		t.Fatal(e)
	}
	tick(t, s, "2026-10-01T09:01")
	before := snap(t, s).Tasks[0]
	c, e := s.Complete(task.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.UndoCompletion(c.Token); e != nil {
		t.Fatal(e)
	}
	after := snap(t, s).Tasks[0]
	if after.Status != "pending" || after.ReminderAt != before.ReminderAt || after.NotifiedAt != before.NotifiedAt {
		t.Fatal(after)
	}
	tick(t, s, "2026-10-01T09:02")
	if len(snap(t, s).Notifications) != 1 {
		t.Fatal("reminder redelivered")
	}
	if e = s.UndoCompletion(c.Token); e == nil {
		t.Fatal("token reused")
	}
	c, e = s.Complete(task.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ToggleImportant(task.ID); e != nil {
		t.Fatal(e)
	}
	if e = s.UndoCompletion(c.Token); e == nil {
		t.Fatal("concurrent edit overwritten")
	}
}
func TestShortcutValidation(t *testing.T) {
	for _, s := range []string{"", "Ctrl+Alt+N", "Alt+Shift+F11", "Ctrl+1", "Ctrl+Alt+Space", "Alt+Home", "Ctrl+NumpadAdd", "Ctrl+F24"} {
		if _, _, e := ParseShortcut(s); e != nil {
			t.Fatal(s, e)
		}
	}
	for _, s := range []string{"N", "Shift+N", "Ctrl+Ctrl+N", "Win+N", "Ctrl+F12", "Ctrl+F1junk", "Ctrl+"} {
		if _, _, e := ParseShortcut(s); e == nil {
			t.Fatal("accepted", s)
		}
	}
}
func TestLargeBackupCanRestoreFromFile(t *testing.T) {
	source := setup(t, "2026-10-01T09:00")
	// Cross both previous limits, without holding attachment data in memory.
	randomPath := filepath.Join(source.Dir, "images", "random.png")
	f, e := os.Create(randomPath)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = io.CopyN(f, rand.Reader, 52<<20); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	f, e = os.Create(filepath.Join(source.Dir, "images", "large.png"))
	if e != nil {
		t.Fatal(e)
	}
	chunk := make([]byte, 1<<20)
	for i := 0; i < 150; i++ {
		if _, e = f.Write(chunk); e != nil {
			t.Fatal(e)
		}
	}
	f.Close()
	task, e := source.SaveTask(Task{Title: "画像付き", Status: "pending", Memo: `<img src="/images/random.png"><img src="/images/large.png">`})
	if e != nil {
		t.Fatal(e)
	}
	zipPath, e := source.Backup()
	if e != nil {
		t.Fatal(e)
	}
	info, e := os.Stat(zipPath)
	if e != nil || info.Size() <= 50<<20 {
		t.Fatal("test did not cross the old ZIP limit", e)
	}
	target, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer target.Close()
	if e = target.RestoreFile(zipPath); e != nil {
		t.Fatal(e)
	}
	if snap(t, target).Tasks[0].Memo != task.Memo {
		t.Fatal("image references changed")
	}
	digest := func(path string) [32]byte {
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		h := sha256.New()
		if _, e = io.Copy(h, f); e != nil {
			t.Fatal(e)
		}
		var sum [32]byte
		copy(sum[:], h.Sum(nil))
		return sum
	}
	for _, name := range []string{"random.png", "large.png"} {
		if digest(filepath.Join(source.Dir, "images", name)) != digest(filepath.Join(target.Dir, "images", name)) {
			t.Fatal("attachment differs", name)
		}
	}
}
