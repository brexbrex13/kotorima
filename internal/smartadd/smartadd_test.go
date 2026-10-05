package smartadd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"kotorima/internal/board"
	"kotorima/internal/jev"
	"testing"
)

func snap() board.Snapshot {
	return board.Snapshot{
		Categories: []board.Category{
			{ID: 3, Name: "家", SortOrder: 2},
			{ID: 1, Name: "仕事", SortOrder: 1},
			{ID: 2, Name: "休眠", SortOrder: 0, Dormant: true},
			{ID: 4, Name: "仕事", SortOrder: 3},
			{ID: 5, Name: "  ", SortOrder: 4},
		},
		Tasks: []board.Task{
			{ID: 10, Title: "古い", Status: "pending", CreatedAt: "2026-01-01T00:00:00"},
			{ID: 11, Title: "請求書提出", Status: "pending", CreatedAt: "2026-02-01T00:00:00"},
			{ID: 12, Title: "完了済み", Status: "done", CreatedAt: "2026-03-01T00:00:00"},
			{ID: 13, Title: "削除済み", Status: "pending", DeletedAt: "2026-03-02T00:00:00", CreatedAt: "2026-03-01T00:00:00"},
		},
	}
}

func criteria(t *testing.T, q jev.Question) []string {
	t.Helper()
	var keys []string
	dec := json.NewDecoder(bytes.NewReader(q.Criteria))
	dec.Token()
	for dec.More() {
		k, _ := dec.Token()
		keys = append(keys, k.(string))
		var skip any
		dec.Decode(&skip)
	}
	return keys
}

func TestBuildCategoriesSkipDormantBlankAndDuplicateNames(t *testing.T) {
	r := Build(snap(), " 請求書を経理に ", MaxOpenTasks, true)
	if fmt.Sprint(r.State.Categories) != "[仕事 家]" || r.State.Input != "請求書を経理に" {
		t.Fatal(r.State)
	}
	if got := fmt.Sprint(criteria(t, r.Questions["category"])); got != "[仕事 家 none_of_these]" {
		t.Fatal(got)
	}
	if r.categories["仕事"] != 1 {
		t.Fatal(r.categories)
	}
}

func TestBuildOmitsCategoryQuestionWithOneCategory(t *testing.T) {
	s := snap()
	s.Categories = s.Categories[:1]
	if _, ok := Build(s, "x", MaxOpenTasks, true).Questions["category"]; ok {
		t.Fatal("category asked with one category")
	}
}

func TestBuildCapsCategoriesAt254(t *testing.T) {
	s := board.Snapshot{}
	for i := 1; i <= 300; i++ {
		s.Categories = append(s.Categories, board.Category{ID: int64(i), Name: fmt.Sprintf("c%d", i), SortOrder: i})
	}
	if n := len(criteria(t, Build(s, "x", MaxOpenTasks, true).Questions["category"])); n != 255 {
		t.Fatal(n)
	}
}

func TestBuildOpenTasksNewestFirstAndLimited(t *testing.T) {
	r := Build(snap(), "別の用事", 1, true)
	if fmt.Sprint(r.State.OpenTasks) != "[請求書提出]" {
		t.Fatal(r.State.OpenTasks)
	}
	if got := fmt.Sprint(criteria(t, r.Questions["duplicate"])); got != "[t11 none]" {
		t.Fatal(got)
	}
	if fmt.Sprint(Build(snap(), "別の用事", MaxOpenTasks, true).State.OpenTasks) != "[請求書提出 古い]" {
		t.Fatal("order")
	}
}

func TestBuildOmitsDuplicateQuestionWithoutOpenTasks(t *testing.T) {
	s := snap()
	s.Tasks = nil
	if _, ok := Build(s, "x", MaxOpenTasks, true).Questions["duplicate"]; ok {
		t.Fatal("duplicate asked without tasks")
	}
}

func TestExactTitleMatchIsFoundLocallyEvenBeyondLimit(t *testing.T) {
	r := Build(snap(), " 古い ", 1, true)
	if _, ok := r.Questions["duplicate"]; ok {
		t.Fatal("asked although exact match is known")
	}
	s := Decide(r, nil)
	if s.Duplicate == nil || s.Duplicate.TaskID != 10 {
		t.Fatal(s.Duplicate)
	}
}

func TestDecideAppliesThresholdsAndEscapes(t *testing.T) {
	r := Build(snap(), "請求書を経理に", MaxOpenTasks, true)
	s := Decide(r, map[string]jev.Answer{
		"category":  {Choice: "仕事", Confidence: CategoryMin},
		"important": {Noul: ImportantMin},
		"deadline":  {Choice: "tomorrow", Confidence: DeadlineMin},
		"reminder":  {Choice: "deadline", Confidence: ReminderMin},
		"duplicate": {Choice: "t11", Confidence: DuplicateMin},
	})
	if s.CategoryID != 1 || !s.Important || s.Deadline != "tomorrow" || s.Reminder != "deadline" || s.Duplicate == nil || s.Duplicate.Title != "請求書提出" {
		t.Fatalf("%+v", s)
	}
	s = Decide(r, map[string]jev.Answer{
		"category":  {Choice: "仕事", Confidence: CategoryMin - 0.01},
		"important": {Noul: ImportantMin - 0.01},
		"deadline":  {Choice: "tomorrow", Confidence: DeadlineMin - 0.01},
		"reminder":  {Choice: "1h", Confidence: ReminderMin - 0.01},
		"duplicate": {Choice: "t11", Confidence: DuplicateMin - 0.01},
	})
	if s != (Suggestion{}) {
		t.Fatalf("%+v", s)
	}
	s = Decide(r, map[string]jev.Answer{
		"category":  {Choice: "none_of_these", Confidence: 1},
		"deadline":  {Choice: "none", Confidence: 1},
		"reminder":  {Choice: "off", Confidence: 1},
		"duplicate": {Choice: "none", Confidence: 1},
	})
	if s != (Suggestion{}) {
		t.Fatalf("%+v", s)
	}
}

func TestDecideIgnoresUnknownChoicesAndOrphanDeadlineReminder(t *testing.T) {
	r := Build(snap(), "x", MaxOpenTasks, true)
	s := Decide(r, map[string]jev.Answer{
		"category":  {Choice: "存在しない", Confidence: 1},
		"deadline":  {Choice: "yesterday", Confidence: 1},
		"reminder":  {Choice: "deadline", Confidence: 1},
		"duplicate": {Choice: "t999", Confidence: 1},
	})
	if s != (Suggestion{}) {
		t.Fatalf("%+v", s)
	}
}

func TestBuildWithoutCategorySendsNoCategories(t *testing.T) {
	r := Build(snap(), "x", MaxOpenTasks, false)
	if _, ok := r.Questions["category"]; ok || len(r.State.Categories) != 0 {
		t.Fatal(r.State.Categories)
	}
	if s := Decide(r, map[string]jev.Answer{"category": {Choice: "仕事", Confidence: 1}}); s.CategoryID != 0 {
		t.Fatal(s.CategoryID)
	}
}

func TestGeneratedChoicesUseValidationWithoutCalibratedConfidence(t *testing.T) {
	r := Build(snap(), "明日までに提出", MaxOpenTasks, true)
	s := Decide(r, map[string]jev.Answer{"deadline": {Choice: "tomorrow", Generated: true}, "category": {Choice: "unknown", Generated: true}, "reminder": {Choice: "invalid", Generated: true}})
	if s.Deadline != "tomorrow" || s.CategoryID != 0 || s.Reminder != "" {
		t.Fatal(s)
	}
}
