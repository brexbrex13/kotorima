package board

import "testing"

func TestNoticeMemoOptionPersistsAndRecurringTasksInherit(t *testing.T) {
	s := setup(t, "2026-09-29T09:05")
	plain, e := s.SaveTask(Task{Title: "default"})
	if e != nil || plain.ShowMemoInNotice {
		t.Fatal(plain, e)
	}
	rule := weekly()
	rule.Memo = `<p>作業場所</p><a href="file:///C:/Work">開く</a>`
	rule.ShowMemoInNotice = true
	saved, e := s.SaveSeries(rule)
	if e != nil {
		t.Fatal(e)
	}
	tick(t, s, "2026-09-29T09:05")
	var task Task
	for _, v := range snap(t, s).Tasks {
		if v.SeriesID == saved.ID {
			task = v
		}
	}
	if task.ID == 0 || !task.ShowMemoInNotice || task.Memo != rule.Memo {
		t.Fatal(task)
	}
	task.ShowMemoInNotice = false
	if _, e = s.SaveTask(task); e != nil {
		t.Fatal(e)
	}
	// Reopening proves the option belongs to each persisted task/template.
	reopened, e := Open(s.Dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	v := snap(t, reopened)
	if !v.Series[0].ShowMemoInNotice {
		t.Fatal(v.Series)
	}
	for _, current := range v.Tasks {
		if current.ShowMemoInNotice {
			t.Fatal(current)
		}
	}
}
