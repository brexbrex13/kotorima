package board

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"strings"
	"time"
)

// Empty means disabled. F12 is reserved by Windows for the debugger;
// Windows validates actual availability when registering the shortcut.
var shortcutKeys = map[string]uint32{
	"Space": 0x20, "Tab": 0x09, "Enter": 0x0D, "Escape": 0x1B, "Backspace": 0x08,
	"Insert": 0x2D, "Delete": 0x2E, "Home": 0x24, "End": 0x23, "PageUp": 0x21, "PageDown": 0x22,
	"Left": 0x25, "Up": 0x26, "Right": 0x27, "Down": 0x28,
	"Numpad0": 0x60, "Numpad1": 0x61, "Numpad2": 0x62, "Numpad3": 0x63, "Numpad4": 0x64,
	"Numpad5": 0x65, "Numpad6": 0x66, "Numpad7": 0x67, "Numpad8": 0x68, "Numpad9": 0x69,
	"NumpadMultiply": 0x6A, "NumpadAdd": 0x6B, "NumpadSubtract": 0x6D, "NumpadDecimal": 0x6E, "NumpadDivide": 0x6F,
}

func ParseShortcut(v string) (uint32, uint32, error) {
	if v == "" {
		return 0, 0, nil
	}
	parts := strings.Split(v, "+")
	var mods, key uint32
	for i, p := range parts {
		if i == len(parts)-1 {
			if len(p) == 1 && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= '0' && p[0] <= '9')) {
				key = uint32(p[0])
			}
			if strings.HasPrefix(p, "F") {
				var n int
				if _, e := fmt.Sscanf(p, "F%d", &n); e == nil && n >= 1 && n <= 24 && n != 12 && p == fmt.Sprintf("F%d", n) {
					key = uint32(0x70 + n - 1)
				}
			}
			if vk, ok := shortcutKeys[p]; ok {
				key = vk
			}
			continue
		}
		var bit uint32
		switch p {
		case "Ctrl":
			bit = 2
		case "Alt":
			bit = 1
		case "Shift":
			bit = 4
		default:
			return 0, 0, errors.New("ショートカットはCtrl・Alt・Shiftと対応するキーを組み合わせてください")
		}
		if mods&bit != 0 {
			return 0, 0, errors.New("修飾キーが重複しています")
		}
		mods |= bit
	}
	if key == 0 || mods&3 == 0 {
		return 0, 0, errors.New("ショートカットにはCtrlまたはAltと、対応するキーを指定してください")
	}
	return mods, key, nil
}

func resolveReminder(t *Task) error {
	if t.ReminderMode == "" {
		return nil
	}
	if t.ReminderMode != "deadline" {
		return errors.New("通知方法が不正です")
	}
	if t.Deadline == "" {
		return errors.New("期限に合わせて通知するには期限日を設定してください")
	}
	if _, e := time.Parse("15:04", t.ReminderTime); e != nil {
		return errors.New("期限日の通知時刻を指定してください")
	}
	if len(t.Deadline) < 10 {
		return errors.New("期限日が不正です")
	}
	t.ReminderAt = t.Deadline[:10] + "T" + t.ReminderTime + ":00"
	return nil
}
func (s *Store) ToggleToday(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := load[Task](s.db, "tasks", id)
	if e != nil {
		return e
	}
	if t.Status != "pending" || t.DeletedAt != "" {
		return errors.New("未完了のタスクを選んでください")
	}
	if t.TodayDate == Date(s.now()) {
		t.TodayDate = ""
	} else {
		t.TodayDate = Date(s.now())
	}
	t.Version++
	return put(s.db, "tasks", id, t)
}

type Completion struct {
	Token  string `json:"token"`
	Title  string `json:"title"`
	TaskID int64  `json:"task_id"`
}
type completionUndo struct {
	Before  Task
	Version int
	Expires time.Time
}

func (s *Store) Complete(id int64) (Completion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, e := load[Task](s.db, "tasks", id)
	if e != nil {
		return Completion{}, e
	}
	if t.Status != "pending" || t.DeletedAt != "" {
		return Completion{}, errors.New("タスクは既に完了・削除されています")
	}
	before := t
	t.Status = "done"
	t.DoneAt = ISO(s.now())
	t.Version++
	if e = put(s.db, "tasks", id, t); e != nil {
		return Completion{}, e
	}
	if s.undo == nil {
		s.undo = map[string]completionUndo{}
	}
	for key, u := range s.undo {
		if !u.Expires.After(s.now()) {
			delete(s.undo, key)
		}
	}
	token := uuid.NewString()
	s.undo[token] = completionUndo{before, t.Version, s.now().Add(time.Minute)}
	return Completion{token, t.Title, t.ID}, nil
}
func (s *Store) UndoCompletion(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.undo[token]
	if !ok || !u.Expires.After(s.now()) {
		return errors.New("取り消しの有効時間が過ぎました。完了済み一覧から再開できます")
	}
	t, e := load[Task](s.db, "tasks", u.Before.ID)
	if e != nil {
		return e
	}
	if t.Version != u.Version || t.Status != "done" || t.DeletedAt != "" {
		return errors.New("完了後に変更されたため取り消せません。完了済み一覧から再開してください")
	}
	// Restore the original notification marker too: an already delivered reminder
	// must not fire again, whereas a future reminder remains scheduled.
	t.Status = u.Before.Status
	t.DoneAt = u.Before.DoneAt
	t.ReminderAt = u.Before.ReminderAt
	t.NotifiedAt = u.Before.NotifiedAt
	t.ReminderMode = u.Before.ReminderMode
	t.ReminderTime = u.Before.ReminderTime
	t.Version++
	if e = put(s.db, "tasks", t.ID, t); e != nil {
		return e
	}
	delete(s.undo, token)
	return nil
}
