package board

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrashCompletedKeepsImagesAndPendingTasks(t *testing.T) {
	s := setup(t, "2026-10-05T09:00")
	path := filepath.Join(s.Dir, "images", "done.png")
	if e := os.WriteFile(path, []byte("image"), 0600); e != nil {
		t.Fatal(e)
	}
	done, e := s.SaveTask(Task{Title: "done", Status: "done", Memo: `<img src="/images/done.png">`})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveTask(Task{Title: "pending"}); e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveTask(Task{Title: "skipped", Status: "skipped"}); e != nil {
		t.Fatal(e)
	}
	if n, e := s.TrashCompleted(); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	if n, e := s.TrashCompleted(); e != nil || n != 0 {
		t.Fatal(n, e)
	}
	if _, e = os.Stat(path); e != nil {
		t.Fatal("trash removed image", e)
	}
	if e = s.SetState(done.ID, "restore"); e != nil {
		t.Fatal(e)
	}
	for _, task := range snap(t, s).Tasks {
		if task.DeletedAt != "" {
			t.Fatal(task)
		}
	}
}

func TestEmptyTrashDeletesOnlyUnsharedInternalImages(t *testing.T) {
	s := setup(t, "2026-10-05T09:00")
	for _, name := range []string{"unique.png", "shared.png", "series.png", "unrelated.png"} {
		if e := os.WriteFile(filepath.Join(s.Dir, "images", name), []byte("image"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	external := filepath.Join(t.TempDir(), "outside.png")
	if e := os.WriteFile(external, []byte("image"), 0600); e != nil {
		t.Fatal(e)
	}
	task, e := s.SaveTask(Task{Title: "removed", ReminderAt: "2026-10-05T08:00", Memo: `<img src="/images/unique.png"><img src='/images/shared.png'><img src="/images/series.png"><img src="file://` + external + `"><img src="/images/../outside.png"><img src="https://example.com/images/unrelated.png">`})
	if e != nil {
		t.Fatal(e)
	}
	tick(t, s, "2026-10-05T09:00")
	completion, e := s.Complete(task.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveTask(Task{Title: "keep", Memo: `<img src="/images/shared.png">`}); e != nil {
		t.Fatal(e)
	}
	rule := weekly()
	rule.Memo = `<img src="/images/series.png">`
	if _, e = s.SaveSeries(rule); e != nil {
		t.Fatal(e)
	}
	if _, e = s.TrashCompleted(); e != nil {
		t.Fatal(e)
	}
	if n, e := s.EmptyTrash(); e != nil || n != 1 {
		t.Fatal(n, e)
	}
	if _, e = os.Stat(filepath.Join(s.Dir, "images", "unique.png")); !os.IsNotExist(e) {
		t.Fatal("unique image retained", e)
	}
	for _, name := range []string{"shared.png", "series.png", "unrelated.png"} {
		if _, e = os.Stat(filepath.Join(s.Dir, "images", name)); e != nil {
			t.Fatal(name, e)
		}
	}
	if _, e = os.Stat(external); e != nil {
		t.Fatal("external file removed", e)
	}
	if len(snap(t, s).Notifications) != 0 {
		t.Fatal("orphan notification")
	}
	if e = s.UndoCompletion(completion.Token); e == nil {
		t.Fatal("purged task can be restored")
	}
	if n, e := s.EmptyTrash(); e != nil || n != 0 {
		t.Fatal(n, e)
	}
}

func TestSuggestedCategoryFallbackDoesNotHideManualErrors(t *testing.T) {
	s := setup(t, "2026-10-05T09:00")
	defaultID := snap(t, s).Categories[0].ID
	if _, e := s.SaveTask(Task{Title: "manual", CategoryID: 999}); e == nil {
		t.Fatal("manual category silently replaced")
	}
	task, e := s.SaveSuggestedTask(Task{Title: "AI", CategoryID: 999}, true)
	if e != nil || task.CategoryID != defaultID {
		t.Fatal(task, e)
	}
	c, e := s.SaveCategory(Category{Name: "hidden", Color: "#fffdf8", TextColor: "#302d25", Dormant: true})
	if e != nil {
		t.Fatal(e)
	}
	task, e = s.SaveSuggestedTask(Task{Title: "AI hidden", CategoryID: c.ID}, true)
	if e != nil || task.CategoryID != defaultID {
		t.Fatal(task, e)
	}
}
