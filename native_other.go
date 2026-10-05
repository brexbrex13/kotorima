//go:build !windows || server

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"os/exec"
	"runtime"
	"time"
)

func noticeStyle() int                                    { return 0 }
func showNotice(w *application.WebviewWindow)             { w.Show() }
func noticeSound()                                        {}
func releaseNoticeNative()                                {}
func activeScreen(a *application.App) *application.Screen { return a.Screen.GetPrimary() }

func openFile(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", path).Start()
	}
	return exec.Command("xdg-open", path).Start()
}
func forgetNotice(w *application.WebviewWindow) {}
func doubleClickDelay() time.Duration           { return 500 * time.Millisecond }

func hideNotice(w *application.WebviewWindow) { w.Hide() }
