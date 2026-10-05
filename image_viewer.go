package main

import (
	"errors"
	"net/url"
	"path"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func imageViewerURL(src string) (string, error) {
	u, err := url.Parse(src)
	if err != nil {
		return "", errors.New("画像のURLが不正です")
	}
	internal := u.Scheme == "" && u.Host == "" && strings.HasPrefix(u.Path, "/images/") &&
		path.Base(u.Path) != "." && path.Base(u.Path) != ".." && strings.TrimPrefix(u.Path, "/images/") == path.Base(u.Path) &&
		!strings.ContainsAny(u.Path, "\\:")
	remote := (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
	if !internal && !remote {
		return "", errors.New("添付画像またはHTTP/HTTPSの画像を指定してください")
	}
	return "/?window=image-viewer&src=" + url.QueryEscape(src), nil
}

func (a *App) OpenImageViewer(src string) error {
	target, err := imageViewerURL(src)
	if err != nil {
		return err
	}
	a.imageViewerMu.Lock()
	defer a.imageViewerMu.Unlock()
	if a.imageViewer == nil {
		width, height := 900, 650
		screen := activeScreen(a.desktop)
		if screen == nil {
			screen = a.desktop.Screen.GetPrimary()
		}
		if screen != nil {
			width = min(width, screen.WorkArea.Width-24)
			height = min(height, screen.WorkArea.Height-24)
		}
		a.imageViewer = a.desktop.Window.NewWithOptions(application.WebviewWindowOptions{
			Name: "image-viewer", Title: "Kotorima 画像", Width: width, Height: height,
			MinWidth: 360, MinHeight: 240, Hidden: true, Frameless: true, AlwaysOnTop: true,
			URL: target, BackgroundColour: application.NewRGB(30, 30, 30),
			Windows: application.WindowsWindow{NonClientRegionSupport: true},
		})
		if screen != nil {
			a.imageViewer.SetPosition(screen.WorkArea.X+(screen.WorkArea.Width-width)/2, screen.WorkArea.Y+(screen.WorkArea.Height-height)/2)
		}
		a.imageViewer.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
			if !a.quitting.Load() {
				e.Cancel()
				a.imageViewer.Hide()
			}
		})
	} else {
		a.imageViewer.SetURL(target)
	}
	a.imageViewer.Show()
	a.imageViewer.Focus()
	return nil
}

func (a *App) CloseImageViewer() {
	a.imageViewerMu.Lock()
	defer a.imageViewerMu.Unlock()
	if a.imageViewer != nil {
		a.imageViewer.Hide()
	}
}
