package board

import "testing"

func TestDefaultCategoryInvariants(t *testing.T) {
	s := setup(t, "2026-10-02T10:00")
	first := snap(t, s).Categories[0]
	first.Name = "仕事"
	first.Color = "#fff0aa"
	if _, e := s.SaveCategory(first); e != nil {
		t.Fatal(e)
	}
	if e := s.DeleteCategory(first.ID); e == nil {
		t.Fatal("last category deleted")
	}
	first.Dormant = true
	if _, e := s.SaveCategory(first); e == nil {
		t.Fatal("default hidden")
	}
	second, e := s.SaveCategory(Category{Name: "生活", Color: "#dcebd5"})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ReorderCategories([]int64{second.ID, first.ID}); e != nil {
		t.Fatal(e)
	}
	task, e := s.SaveTask(Task{Title: "新しい追加先"})
	if e != nil || task.CategoryID != second.ID {
		t.Fatal(task, e)
	}
	if e = s.DeleteCategory(second.ID); e != nil {
		t.Fatal(e)
	}
	if snap(t, s).Tasks[0].CategoryID != first.ID {
		t.Fatal("orphaned task")
	}
	path, e := s.Backup()
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RestoreFile(path); e != nil {
		t.Fatal(e)
	}
	if e = s.RestoreFile(path); e != nil {
		t.Fatal(e)
	}
	if len(snap(t, s).Categories) != 1 {
		t.Fatal("restoration multiplied categories")
	}
}

func TestLegacyVirtualCategoryUpgradesOnce(t *testing.T) {
	s := setup(t, "2026-10-02T10:00")
	task, e := s.SaveTask(Task{Title: "旧未分類のメモ", Memo: "本文を維持"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("DELETE FROM metadata WHERE key='category_model'"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("UPDATE tasks SET data=json_set(data,'$.category_id',0) WHERE id=?", task.ID); e != nil {
		t.Fatal(e)
	}
	dir := s.Dir
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	v := snap(t, reopened)
	first, e := defaultCategory(reopened.db)
	if e != nil {
		t.Fatal(e)
	}
	if v.Tasks[0].CategoryID != first || v.Tasks[0].Memo != "本文を維持" {
		t.Fatal(v.Tasks)
	}
	count := len(v.Categories)
	if e = normalizeCategories(reopened.db); e != nil {
		t.Fatal(e)
	}
	if len(snap(t, reopened).Categories) != count {
		t.Fatal("duplicate migration")
	}
}
