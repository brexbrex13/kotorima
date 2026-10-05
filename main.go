package main

import (
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"io/fs"
	"kotorima/internal/board"
	"kotorima/internal/notify"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed all:frontend/dist build/appicon.png internal/tray/icon.ico
var assets embed.FS

func main() {
	if runNativeProbe() {
		return
	}
	dir := os.Getenv("KOTORIMA_DATA_DIR")
	if dir == "" {
		dir = os.Getenv("MEMOTODO_DATA_DIR")
	}
	if dir == "" {
		exe, e := os.Executable()
		if e != nil {
			panic(e)
		}
		dir = filepath.Join(filepath.Dir(exe), "data")
	}
	store, e := board.Open(dir)
	if e != nil {
		panic(fmt.Errorf("保存先を開けません: %w", e))
	}
	defer store.Close()
	service := &App{store: store, stop: make(chan struct{}), seen: map[int64]bool{}}
	service.jevKeys = newKeyStore(dir)
	frontend, e := fs.Sub(assets, "frontend/dist")
	if e != nil {
		panic(e)
	}
	handler := application.AssetFileServerFS(frontend)
	icon, _ := assets.ReadFile("build/appicon.png")
	service.desktop = application.New(application.Options{Name: "Kotorima", Icon: icon, Description: "付箋・Todo・メモ・リマインダー", Services: []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/images/") {
				name := strings.TrimPrefix(r.URL.Path, "/images/")
				if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, "\\:") {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("X-Content-Type-Options", "nosniff")
				http.ServeFile(w, r, filepath.Join(dir, "images", name))
				return
			}
			handler.ServeHTTP(w, r)
		})},
		SingleInstance: &application.SingleInstanceOptions{UniqueID: "memotodo-sticky-board-v3", OnSecondInstanceLaunch: func(application.SecondInstanceData) { service.openMain(0) }},
	})
	ico, _ := assets.ReadFile("internal/tray/icon.ico")
	width, height := store.WindowSize()
	service.main = service.desktop.Window.NewWithOptions(application.WebviewWindowOptions{Name: "board", Title: "Kotorima", Width: width, Height: height, MinWidth: 360, MinHeight: 420, Frameless: true, Windows: application.WindowsWindow{NonClientRegionSupport: true}, MinimiseButtonState: application.ButtonHidden, MaximiseButtonState: application.ButtonHidden, URL: "/", BackgroundColour: application.NewRGB(247, 246, 241)})
	service.notice = service.desktop.Window.NewWithOptions(application.WebviewWindowOptions{Name: "notifications", Title: "Kotorima 通知", Width: 360, Height: 170, Hidden: true, Frameless: true, AlwaysOnTop: true, DisableResize: true, URL: "/?window=notifications", Windows: application.WindowsWindow{HiddenOnTaskbar: false, ExStyle: noticeStyle()}, BackgroundColour: application.NewRGB(247, 246, 241)})
	service.main.RegisterHook(events.Common.WindowMaximise, func(ev *application.WindowEvent) { ev.Cancel() })
	rememberMain := func(*application.WindowEvent) {
		if !service.mainBoundsReady.Load() {
			return
		}
		if e := service.saveMainWindowBounds(); e != nil {
			service.desktop.Event.Emit("board:error", e.Error())
		}
	}
	service.main.OnWindowEvent(events.Windows.WindowEndMove, rememberMain)
	service.main.OnWindowEvent(events.Windows.WindowEndResize, rememberMain)
	service.main.RegisterHook(events.Common.WindowClosing, func(ev *application.WindowEvent) {
		if service.quitting.Load() {
			return
		}
		ev.Cancel()
		service.desktop.Event.Emit("board:close-request", "hide")
	})
	service.notice.RegisterHook(events.Common.WindowClosing, func(ev *application.WindowEvent) {
		if !service.quitting.Load() {
			ev.Cancel()
		}
	})
	tray := service.desktop.SystemTray.New()
	tray.SetIcon(ico)
	service.mini = service.desktop.Window.NewWithOptions(application.WebviewWindowOptions{Name: "quick-add", Title: "Kotorima タスク追加", Width: 360, Height: 105, Hidden: true, Frameless: true, AlwaysOnTop: true, DisableResize: true, URL: "/?window=quick-add", Windows: application.WindowsWindow{HiddenOnTaskbar: true}, BackgroundColour: application.NewRGB(247, 246, 241)})
	service.quickSuggestions = service.desktop.Window.NewWithOptions(application.WebviewWindowOptions{Name: "quick-suggestions", Title: "Kotorima 入力候補", Width: 336, Height: 178, Hidden: true, Frameless: true, AlwaysOnTop: true, DisableResize: true, URL: "/?window=quick-suggestions", Windows: application.WindowsWindow{HiddenOnTaskbar: true, ExStyle: quickSuggestionStyle()}, BackgroundColour: application.NewRGB(247, 246, 241)})
	service.mini.RegisterHook(events.Common.WindowClosing, func(ev *application.WindowEvent) {
		if !service.quitting.Load() {
			ev.Cancel()
			service.HideQuickAdd()
		}
	})
	service.mini.OnWindowEvent(events.Windows.WindowInactive, func(*application.WindowEvent) { service.quickLostFocus() })
	service.mini.OnWindowEvent(events.Windows.WindowEndMove, func(*application.WindowEvent) { service.rememberQuickPosition() })
	service.mini.OnWindowEvent(events.Windows.WindowEndResize, func(*application.WindowEvent) { service.rememberQuickPosition() })
	tray.AttachWindow(service.mini).WindowOffset(8)
	tray.SetTooltip("Kotorima — クリックでタスク追加／ダブルクリックで一覧")
	service.tray = tray
	tray.OnClick(service.traySingleClick)
	tray.OnDoubleClick(service.trayDoubleClick)
	menu := application.NewMenu()
	menu.Add("ボードを開く").OnClick(func(*application.Context) { service.openMain(0) })
	menu.Add("タスクを追加").OnClick(func(*application.Context) { service.showQuickAdd() })
	menu.AddSeparator()
	menu.Add("終了（保存して終了）").OnClick(func(*application.Context) {
		service.main.Show()
		service.desktop.Event.Emit("board:close-request", "quit")
	})
	tray.SetMenu(menu)
	os.WriteFile(filepath.Join(dir, "notify_icon.png"), icon, 0644)
	notify.Init(filepath.Join(dir, "notify_icon.png"), func() { service.openMain(0) })
	service.desktop.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		service.restoreMainWindowBounds()
		service.mainBoundsReady.Store(true)
		service.shortcut = newShortcutManager(service.showQuickAtCursor)
		if v, e := store.Snapshot(); e == nil {
			if e = service.shortcut.Change(v.Settings.QuickShortcut); e != nil {
				service.settingsMu.Lock()
				service.shortcutError = e.Error()
				service.settingsMu.Unlock()
				service.desktop.Event.Emit("board:error", e.Error())
			}
		}
		service.start()
		startNativeVerification(service)
	})
	service.desktop.OnShutdown(func() {
		service.quitting.Store(true)
		if service.shortcut != nil {
			service.shortcut.Close()
		}
		service.stopOnce.Do(func() { close(service.stop) })
		releaseNoticeNative()
	})
	if e = service.desktop.Run(); e != nil {
		panic(e)
	}
}
