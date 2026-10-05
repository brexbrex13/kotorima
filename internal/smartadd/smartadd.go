// Package smartadd turns a quick-add title into Jev questions and applies
// confidence thresholds to the answers.
package smartadd

import (
	"fmt"
	"kotorima/internal/board"
	"kotorima/internal/jev"
	"sort"
	"strings"
)

const (
	CategoryMin  = 0.80
	ImportantMin = 0.85
	DeadlineMin  = 0.80
	ReminderMin  = 0.85
	DuplicateMin = 0.70
	MaxOpenTasks = 200
	maxChoices   = 255
	noneCategory = "none_of_these"
)

type Dup struct {
	TaskID int64  `json:"task_id"`
	Title  string `json:"title"`
}

type Suggestion struct {
	CategoryID  int64  `json:"category_id"`
	Important   bool   `json:"important"`
	Deadline    string `json:"deadline"`
	Reminder    string `json:"reminder"`
	Duplicate   *Dup   `json:"duplicate"`
	Invalid     bool   `json:"invalid"`
	Unavailable bool   `json:"unavailable"`
}

type State struct {
	Input      string   `json:"input"`
	Categories []string `json:"categories"`
	OpenTasks  []string `json:"open_tasks"`
}

type Request struct {
	State      State
	Questions  map[string]jev.Question
	categories map[string]int64
	tasks      map[string]Dup
	exact      *Dup
}

var deadlineOptions = []jev.Option{
	{Key: "none", Description: "期限の手掛かりが無い"},
	{Key: "today", Description: "今日中"},
	{Key: "tomorrow", Description: "明日まで"},
	{Key: "this_week", Description: "今週中"},
	{Key: "next_week", Description: "来週中"},
}

var reminderOptions = []jev.Option{
	{Key: "off", Description: "通知は要らない"},
	{Key: "deadline", Description: "期限日に通知する"},
	{Key: "1h", Description: "1時間後に通知する"},
	{Key: "4h", Description: "4時間後に通知する"},
	{Key: "tomorrow", Description: "明日の朝に通知する"},
	{Key: "next-week", Description: "来週の初めに通知する"},
}

// askCategory=false leaves category names out of the request entirely.
func Build(s board.Snapshot, title string, limit int, askCategory bool) Request {
	title = strings.TrimSpace(title)
	r := Request{
		State:      State{Input: title, Categories: []string{}, OpenTasks: []string{}},
		Questions:  map[string]jev.Question{},
		categories: map[string]int64{},
		tasks:      map[string]Dup{},
	}
	cats := append([]board.Category(nil), s.Categories...)
	sort.SliceStable(cats, func(i, j int) bool {
		if cats[i].SortOrder != cats[j].SortOrder {
			return cats[i].SortOrder < cats[j].SortOrder
		}
		return cats[i].ID < cats[j].ID
	})
	var catOptions []jev.Option
	for _, c := range cats {
		name := strings.TrimSpace(c.Name)
		if !askCategory || c.Dormant || name == "" || name == noneCategory || len(catOptions) == maxChoices-1 {
			continue
		}
		if _, seen := r.categories[name]; seen {
			continue
		}
		r.categories[name] = c.ID
		r.State.Categories = append(r.State.Categories, name)
		catOptions = append(catOptions, jev.Option{Key: name})
	}
	if len(catOptions) > 1 {
		r.Questions["category"] = jev.Choice("入力されたタスク(input)を入れるカテゴリ。どれにも当てはまらなければ none_of_these。",
			append(catOptions, jev.Option{Key: noneCategory, Description: "どのカテゴリにも当てはまらない"}))
	}
	r.Questions["important"] = jev.Noul("入力されたタスク(input)は重要である。", "締切や他人への影響が大きく、優先して扱うべき用事", "日常的な、通常の用事")
	r.Questions["deadline"] = jev.Choice("入力されたタスク(input)の文面から読み取れる期限。読み取れなければ none。", deadlineOptions)
	r.Questions["reminder"] = jev.Choice("入力されたタスク(input)に付けるべき通知。特に要らなければ off。", reminderOptions)

	var open []board.Task
	for _, t := range s.Tasks {
		if t.Status == "pending" && t.DeletedAt == "" && strings.TrimSpace(t.Title) != "" {
			open = append(open, t)
		}
	}
	sort.SliceStable(open, func(i, j int) bool {
		if open[i].CreatedAt != open[j].CreatedAt {
			return open[i].CreatedAt > open[j].CreatedAt
		}
		return open[i].ID > open[j].ID
	})
	for _, t := range open {
		if strings.EqualFold(strings.TrimSpace(t.Title), title) {
			r.exact = &Dup{TaskID: t.ID, Title: t.Title}
			break
		}
	}
	limit = min(limit, MaxOpenTasks)
	if len(open) > limit {
		open = open[:max(limit, 0)]
	}
	options := make([]jev.Option, 0, len(open)+1)
	for _, t := range open {
		key := fmt.Sprintf("t%d", t.ID)
		r.tasks[key] = Dup{TaskID: t.ID, Title: t.Title}
		r.State.OpenTasks = append(r.State.OpenTasks, t.Title)
		options = append(options, jev.Option{Key: key, Description: t.Title})
	}
	if r.exact == nil && len(options) > 0 {
		r.Questions["duplicate"] = jev.Choice("入力されたタスク(input)と同じ用事を指す、既存の未完了タスク。同じ用事が無ければ none。",
			append(options, jev.Option{Key: "none", Description: "同じ用事のタスクは無い"}))
	}
	return r
}

func Decide(r Request, answers map[string]jev.Answer) Suggestion {
	var s Suggestion
	if a, ok := answers["category"]; ok && (a.Generated || a.Confidence >= CategoryMin) {
		s.CategoryID = r.categories[a.Choice]
	}
	if a, ok := answers["important"]; ok && a.Noul >= ImportantMin {
		s.Important = true
	}
	if a, ok := answers["deadline"]; ok && (a.Generated || a.Confidence >= DeadlineMin) && known(deadlineOptions, a.Choice) && a.Choice != "none" {
		s.Deadline = a.Choice
	}
	if a, ok := answers["reminder"]; ok && (a.Generated || a.Confidence >= ReminderMin) && known(reminderOptions, a.Choice) && a.Choice != "off" {
		s.Reminder = a.Choice
	}
	if s.Reminder == "deadline" && s.Deadline == "" {
		s.Reminder = ""
	}
	if r.exact != nil {
		d := *r.exact
		s.Duplicate = &d
	} else if a, ok := answers["duplicate"]; ok && (a.Generated || a.Confidence >= DuplicateMin) {
		if d, ok := r.tasks[a.Choice]; ok {
			s.Duplicate = &d
		}
	}
	return s
}

func known(options []jev.Option, key string) bool {
	for _, o := range options {
		if o.Key == key {
			return true
		}
	}
	return false
}
