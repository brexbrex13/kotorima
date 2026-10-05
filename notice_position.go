package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"kotorima/internal/board"
	"strings"
)

func noticeSize(n board.Notification, snapshot board.Snapshot) (width, height int) {
	if n.Kind == "summary" {
		return 440, 390
	}
	if !snapshot.Settings.Private {
		for _, t := range snapshot.Tasks {
			if t.ID == n.TaskID && t.ShowMemoInNotice && strings.TrimSpace(t.Memo) != "" {
				return 360, 330
			}
		}
	}
	return 360, 170
}

func noticeScreen(app *application.App, monitor string) *application.Screen {
	s := app.Screen.GetPrimary()
	if monitor == "active" {
		s = activeScreen(app)
	} else if monitor != "primary" {
		if chosen := app.Screen.GetByID(monitor); chosen != nil {
			s = chosen
		}
	}
	return s
}
func positionNotice(app *application.App, w *application.WebviewWindow, monitor string) {
	s := noticeScreen(app, monitor)
	if s == nil {
		return
	}
	r := s.WorkArea
	width, height := 360, 170
	if r.Width-24 < width {
		width = r.Width - 24
	}
	if r.Height-24 < height {
		height = r.Height - 24
	}
	w.SetSize(width, height)
	w.SetPosition(r.X+r.Width-width-12, r.Y+r.Height-height-12)
}
