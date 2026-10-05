package board

import (
	"kotorima/internal/jev"
	"time"
)

type Task struct {
	ID               int64  `json:"id"`
	Version          int    `json:"version"`
	Title            string `json:"title"`
	Memo             string `json:"memo"`
	ShowMemoInNotice bool   `json:"show_memo_in_notice"`
	Status           string `json:"status"`
	Deadline         string `json:"deadline"`
	ReminderAt       string `json:"reminder_at"`
	ReminderMode     string `json:"reminder_mode"`
	ReminderTime     string `json:"reminder_time"`
	TodayDate        string `json:"today_date"`
	NotifiedAt       string `json:"notified_at"`
	CategoryID       int64  `json:"category_id"`
	Important        bool   `json:"important"`
	SortOrder        int64  `json:"sort_order"`
	CreatedAt        string `json:"created_at"`
	DoneAt           string `json:"done_at"`
	DeletedAt        string `json:"deleted_at"`
	SeriesID         int64  `json:"series_id"`
	Occurrence       string `json:"occurrence"`
	NearDays         int    `json:"near_days"`
}
type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
	TextColor string `json:"text_color"`
	Dormant   bool   `json:"dormant"`
}
type ColorPreset struct {
	Name       string `json:"name"`
	Background string `json:"background"`
	Foreground string `json:"foreground"`
}
type Series struct {
	ID               int64  `json:"id"`
	Version          int    `json:"version"`
	Title            string `json:"title"`
	Memo             string `json:"memo"`
	ShowMemoInNotice bool   `json:"show_memo_in_notice"`
	CategoryID       int64  `json:"category_id"`
	Important        bool   `json:"important"`
	Period           string `json:"period"`
	Interval         int    `json:"interval"`
	Weekdays         []int  `json:"weekdays"`
	MonthDay         int    `json:"month_day"` // 0=month end
	Month            int    `json:"month"`
	FirstDue         string `json:"first_due"`
	NextDue          string `json:"next_due"`
	DueTime          string `json:"due_time"`
	ShowDays         int    `json:"show_days"`
	NearDays         int    `json:"near_days"`
	NotifyMode       string `json:"notify_mode"` // off, days, hours
	NotifyDays       int    `json:"notify_days"`
	NotifyTime       string `json:"notify_time"`
	NotifyHours      int    `json:"notify_hours"`
	Active           bool   `json:"active"`
	Deleted          bool   `json:"deleted"`
	EndDate          string `json:"end_date"`
}
type Notification struct {
	ID           int64  `json:"id"`
	Key          string `json:"key"`
	TaskID       int64  `json:"task_id"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	FiredAt      string `json:"fired_at"`
	Acknowledged bool   `json:"acknowledged"`
}
type Settings struct {
	ReminderDefaultTime string        `json:"reminder_default_time"`
	QuickShortcut       string        `json:"quick_shortcut"`
	SuggestMinCount     int           `json:"suggest_min_count"`
	Theme               string        `json:"theme"`
	SeriesShowDays      int           `json:"series_show_days"`
	CustomColors        []ColorPreset `json:"custom_colors"`
	NearDays            int           `json:"near_days"`
	Workdays            bool          `json:"workdays"`
	NotifyTimes         []string      `json:"notify_times"`
	NotifyWeekdays      []int         `json:"notify_weekdays"`
	Sound               bool          `json:"sound"`
	NativeToast         bool          `json:"native_toast"`
	Private             bool          `json:"private"`
	PauseUntil          string        `json:"pause_until"`
	FontSize            int           `json:"font_size"`
	Compact             bool          `json:"compact"`
	Collapsed           []int64       `json:"collapsed"`
	Monitor             string        `json:"monitor"`            // active, primary, screen ID
	AITimeoutSeconds    int           `json:"ai_timeout_seconds"` // 0 selects provider default
	SmartAdd            bool          `json:"smart_add"`
	// Jev endpoint details are not secret; API keys live outside settings.
	JevProvider          string `json:"jev_provider"` // "", typesafe, vercel, cloudflare, custom
	JevCloudflareAccount string `json:"jev_cloudflare_account"`
	JevCustomURL         string `json:"jev_custom_url"`
	JevCustomModel       string `json:"jev_custom_model"`
}

func (v Settings) JevEndpoint() jev.Endpoint {
	return jev.Endpoint{TimeoutSeconds: v.AITimeoutSeconds, Provider: v.JevProvider, Account: v.JevCloudflareAccount, BaseURL: v.JevCustomURL, Model: v.JevCustomModel}
}

type Snapshot struct {
	Tasks         []Task         `json:"tasks"`
	Categories    []Category     `json:"categories"`
	Series        []Series       `json:"series"`
	Notifications []Notification `json:"notifications"`
	Settings      Settings       `json:"settings"`
}

func Defaults() Settings {
	return Settings{ReminderDefaultTime: "09:00", QuickShortcut: "Ctrl+Alt+N", SuggestMinCount: 3, SeriesShowDays: 7, CustomColors: []ColorPreset{}, NearDays: 3, Workdays: true, NotifyTimes: []string{"13:00", "17:00"}, NotifyWeekdays: []int{1, 2, 3, 4, 5}, Sound: true, FontSize: 14, Collapsed: []int64{}, Monitor: "active"}
}
func ISO(t time.Time) string  { return t.Format("2006-01-02T15:04:05") }
func Date(t time.Time) string { return t.Format("2006-01-02") }
func ParseTime(v string, loc *time.Location) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02"} {
		if t, e := time.ParseInLocation(layout, v, loc); e == nil {
			return t, nil
		}
	}
	return time.ParseInLocation("2006-01-02T15:04:05", v, loc)
}
func Due(t Task, loc *time.Location) (time.Time, error) {
	d, e := ParseTime(t.Deadline, loc)
	if e == nil && len(t.Deadline) == 10 {
		d = d.AddDate(0, 0, 1).Add(-time.Second)
	}
	return d, e
}
func Urgency(t Task, now time.Time, settings Settings) string {
	if t.Status != "pending" || t.DeletedAt != "" || t.Deadline == "" {
		return ""
	}
	due, e := Due(t, now.Location())
	if e != nil {
		return ""
	}
	if due.Before(now) {
		return "overdue"
	}
	if Date(due) == Date(now) {
		return "today"
	}
	n := settings.NearDays
	days := 0
	cursor := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	target := Date(due)
	for Date(cursor) < target && days <= n {
		cursor = cursor.AddDate(0, 0, 1)
		if !settings.Workdays || (cursor.Weekday() != time.Saturday && cursor.Weekday() != time.Sunday) {
			days++
		}
	}
	if days <= n {
		return "near"
	}
	return ""
}
