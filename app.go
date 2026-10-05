package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	"kotorima/internal/board"
	"kotorima/internal/jev"
	"kotorima/internal/notify"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type App struct {
	imageViewer          *application.WebviewWindow
	imageViewerMu        sync.Mutex
	shortcut             *shortcutManager
	settingsMu           sync.Mutex
	shortcutError        string
	quickPrevious        atomic.Uintptr
	quickCursor          atomic.Bool
	quickGeneration      atomic.Uint64
	quickX, quickY       int32
	store                *board.Store
	desktop              *application.App
	main, notice         *application.WebviewWindow
	mini                 *application.WebviewWindow
	quickSuggestions     *application.WebviewWindow
	quickSuggestionMu    sync.Mutex
	quickSuggestionData  QuickSuggestions
	quickSuggestionReady atomic.Bool
	tray                 *application.SystemTray
	trayGeneration       atomic.Uint64
	trayDouble           atomic.Int64
	stop                 chan struct{}
	wg                   sync.WaitGroup
	mu                   sync.Mutex
	seen                 map[int64]bool
	noticeWindows        map[int64]*application.WebviewWindow
	noticeLoaded         map[int64]bool
	noticeShown          map[int64]bool
	noticeVisible        bool
	noticeReady          atomic.Bool
	startOnce            sync.Once
	stopOnce             sync.Once
	quitting             atomic.Bool
	jevKeys              keyStore
	newJev               func(ep jev.Endpoint, key string) (jevAPI, error)
	jevMu                sync.Mutex
	jevRevision          uint64
	aiPending            map[string]pendingAI
	aiSequence           uint64
	jevBad               map[string]bool // providers whose saved key was rejected
	aiStopped            map[string]bool // session only; a successful connection check resumes AI
	mainBoundsReady      atomic.Bool
}

func (a *App) start() { a.startOnce.Do(func() { a.wg.Add(1); go a.run() }) }
func (a *App) Ready(window string) {
	if window == "quick-suggestions" {
		a.quickSuggestionReady.Store(true)
		a.refreshQuickSuggestions()
		return
	}
	if strings.HasPrefix(window, "notifications:") {
		id, _ := strconv.ParseInt(strings.TrimPrefix(window, "notifications:"), 10, 64)
		a.mu.Lock()
		if a.noticeLoaded == nil {
			a.noticeLoaded = map[int64]bool{}
		}
		a.noticeLoaded[id] = true
		a.mu.Unlock()
	}
	if window == "notifications" {
		a.noticeReady.Store(true)
	}
	a.start()
	if window == "board" {
		if status := a.GetShortcutStatus(); status != "" {
			a.desktop.Event.Emit("board:error", status)
		}
	}
	a.refreshNotice()
}
func (a *App) run() {
	defer a.wg.Done()
	a.tick(true)
	if e := a.store.DailyBackup(); e != nil {
		log.Printf("backup: %v", e)
		a.desktop.Event.Emit("board:error", "自動バックアップに失敗しました: "+e.Error())
	}
	backupDay := board.Date(time.Now())
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.stop:
			return
		case <-ticker.C:
			a.tick(false)
			if today := board.Date(time.Now()); today != backupDay {
				if e := a.store.DailyBackup(); e != nil {
					a.desktop.Event.Emit("board:error", "自動バックアップに失敗しました: "+e.Error())
				} else {
					backupDay = today
				}
			}
		}
	}
}
func (a *App) tick(startup bool) {
	changed, e := a.store.Tick(time.Now(), startup)
	if e != nil {
		log.Printf("scheduler: %v", e)
		a.desktop.Event.Emit("board:error", "通知処理に失敗しました: "+e.Error())
		return
	}
	if changed {
		a.desktop.Event.Emit("board:changed")
	}
	a.refreshNotice()
}
func (a *App) changed() {
	if _, e := a.store.SyncChanges(time.Now()); e != nil {
		a.desktop.Event.Emit("board:error", "通知処理に失敗しました: "+e.Error())
	}
	a.refreshNotice()
	a.desktop.Event.Emit("board:changed")
}
func (a *App) refreshNotice() {
	if a.quitting.Load() {
		return
	}
	if !a.noticeReady.Load() {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	v, e := a.store.Snapshot()
	if e != nil {
		return
	}
	pending := 0
	fresh := false
	live := []board.Notification{}
	for _, n := range v.Notifications {
		if !n.Acknowledged {
			live = append(live, n)
			pending++
			if !a.seen[n.ID] {
				fresh = true
			}
		}
	}
	paused := false
	if v.Settings.PauseUntil != "" {
		p, e := board.ParseTime(v.Settings.PauseUntil, time.Local)
		paused = e == nil && p.After(time.Now())
	}
	if a.noticeWindows == nil {
		a.noticeWindows = map[int64]*application.WebviewWindow{}
		a.noticeShown = map[int64]bool{}
		if a.noticeLoaded == nil {
			a.noticeLoaded = map[int64]bool{}
		}
	}
	for id, w := range a.noticeWindows {
		found := false
		for _, n := range live {
			if n.ID == id {
				found = true
				break
			}
		}
		if !found {
			hideNotice(w)
			forgetNotice(w)
			w.Close()
			delete(a.noticeWindows, id)
			delete(a.noticeLoaded, id)
			delete(a.noticeShown, id)
		}
	}
	if pending == 0 || paused {
		if a.noticeVisible {
			hideNotice(a.notice)
			a.noticeVisible = false
		}
		for id, w := range a.noticeWindows {
			hideNotice(w)
			a.noticeShown[id] = false
		}
		return
	}
	// Each reminder owns a small window. Place them vertically, then in adjacent
	// columns. Overflow stays durable and is promoted when a visible item closes.
	offset, column := 0, 0
	visibleIDs := map[int64]bool{}
	screen := noticeScreen(a.desktop, v.Settings.Monitor)
	if screen == nil {
		return
	}
	for i, n := range live {
		if i == 0 {
			if previous := a.noticeWindows[n.ID]; previous != nil {
				hideNotice(previous)
				forgetNotice(previous)
				previous.Close()
				delete(a.noticeWindows, n.ID)
				delete(a.noticeLoaded, n.ID)
				delete(a.noticeShown, n.ID)
			}
		}
		width, height := noticeSize(n, v)
		if height > screen.WorkArea.Height-24 {
			height = screen.WorkArea.Height - 24
		}
		if width > screen.WorkArea.Width-24 {
			width = screen.WorkArea.Width - 24
		}
		if offset > 0 && offset+height > screen.WorkArea.Height-24 {
			offset = 0
			column++
		}
		if column > 0 && (column+1)*452 > screen.WorkArea.Width {
			break
		}
		visibleIDs[n.ID] = true
		var w *application.WebviewWindow
		if i == 0 {
			w = a.notice
		} else {
			w = a.noticeWindows[n.ID]
			if w == nil {
				w = a.desktop.Window.NewWithOptions(application.WebviewWindowOptions{Name: fmt.Sprintf("notice-%d", n.ID), Title: "Kotorima 通知", Width: width, Height: height, Hidden: true, Frameless: true, AlwaysOnTop: true, DisableResize: true, URL: fmt.Sprintf("/?window=notifications&notice=%d", n.ID), Windows: application.WindowsWindow{HiddenOnTaskbar: false, ExStyle: noticeStyle()}, BackgroundColour: application.NewRGB(255, 250, 240)})
				a.noticeWindows[n.ID] = w
			}
		}
		w.SetBounds(application.Rect{X: screen.WorkArea.X + screen.WorkArea.Width - width - 12 - column*452, Y: screen.WorkArea.Y + screen.WorkArea.Height - height - 12 - offset, Width: width, Height: height})
		if i == 0 {
			if !a.noticeVisible {
				showNotice(w)
				a.noticeVisible = true
			}
		} else if a.noticeLoaded[n.ID] && !a.noticeShown[n.ID] {
			showNotice(w)
			a.noticeShown[n.ID] = true
		}
		offset += height + 10
	}
	for id, w := range a.noticeWindows {
		if !visibleIDs[id] && a.noticeShown[id] {
			hideNotice(w)
			a.noticeShown[id] = false
		}
	}
	if fresh {
		for _, n := range v.Notifications {
			if !n.Acknowledged {
				a.seen[n.ID] = true
			}
		}
		if v.Settings.Sound {
			noticeSound()
		}
		if v.Settings.NativeToast {
			body := fmt.Sprintf("未確認の通知が%d件あります", pending)
			if !v.Settings.Private {
				for _, n := range v.Notifications {
					if !n.Acknowledged {
						body = n.Title
						break
					}
				}
			}
			notify.Push("Kotorima", body)
		}
	}
}
func (a *App) openMain(id int64) {
	if a.mini != nil {
		a.hideQuickSuggestions()
		a.mini.Hide()
	}
	a.main.Show()
	a.main.UnMinimise()
	a.ensureMainVisible()
	a.main.Focus()
	if id != 0 {
		a.desktop.Event.Emit("board:open", id)
	}
}
func (a *App) GetSnapshot() (board.Snapshot, error) { return a.store.Snapshot() }
func (a *App) SaveSuggestedTask(v board.Task, automaticCategory bool) (board.Task, error) {
	t, e := a.store.SaveSuggestedTask(v, automaticCategory)
	if e == nil {
		a.changed()
	}
	return t, e
}
func (a *App) TrashCompleted() (int, error) {
	n, e := a.store.TrashCompleted()
	if n > 0 {
		a.changed()
	}
	return n, e
}
func (a *App) EmptyTrash() (int, error) {
	n, e := a.store.EmptyTrash()
	if n > 0 {
		a.changed()
	}
	return n, e
}
func (a *App) SaveTask(v board.Task) (board.Task, error) {
	t, e := a.store.SaveTask(v)
	if e == nil {
		a.changed()
	}
	return t, e
}
func (a *App) SetState(id int64, state string) error {
	if state == "done" {
		c, e := a.store.Complete(id)
		if e != nil {
			return e
		}
		a.changed()
		a.desktop.Event.Emit("board:completed", c)
		return nil
	}
	e := a.store.SetState(id, state)
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) Reorder(ids []int64) error {
	e := a.store.Reorder(ids)
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) SaveCategory(c board.Category) (board.Category, error) {
	v, e := a.store.SaveCategory(c)
	if e == nil {
		a.changed()
	}
	return v, e
}
func (a *App) DeleteCategory(id int64) error {
	e := a.store.DeleteCategory(id)
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) PreviewSeries(v board.Series) (board.SeriesPreview, error) {
	return a.store.PreviewSeries(v)
}
func (a *App) SaveSeries(v board.Series) (board.Series, error) {
	s, e := a.store.SaveSeries(v)
	if e == nil {
		a.changed()
	}
	return s, e
}
func (a *App) StopSeries(id int64) error {
	e := a.store.StopSeries(id)
	if e == nil {
		a.changed()
	}
	return e
}

// Change only the category display preference, without overwriting settings drafts.
func (a *App) SetCategoryCollapsed(id int64, collapsed bool) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	v, err := a.store.Snapshot()
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(v.Settings.Collapsed)+1)
	found := false
	for _, c := range v.Categories {
		if c.ID == id {
			found = true
			break
		}
	}
	for _, existing := range v.Settings.Collapsed {
		if existing != id {
			ids = append(ids, existing)
		}
	}
	if collapsed && found {
		ids = append(ids, id)
	}
	v.Settings.Collapsed = ids
	if err = a.store.SaveSettings(v.Settings); err != nil {
		return err
	}
	a.changed()
	return nil
}

func (a *App) SaveSettings(v board.Settings) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	old, e := a.store.Snapshot()
	if e != nil {
		return e
	}
	if e = a.store.ValidateSettings(v); e != nil {
		return e
	}
	shortcutChanged := v.QuickShortcut != old.Settings.QuickShortcut
	if a.shortcut != nil && shortcutChanged {
		if e = a.shortcut.Change(v.QuickShortcut); e != nil {
			return e
		}
	}
	e = a.store.SaveSettings(v)
	if e != nil {
		if a.shortcut != nil && shortcutChanged {
			a.shortcut.Change(old.Settings.QuickShortcut)
		}
		return e
	}
	if shortcutChanged {
		a.shortcutError = ""
	}
	if v.SmartAdd != old.Settings.SmartAdd || v.JevEndpoint() != old.Settings.JevEndpoint() || v.AITimeoutSeconds != old.Settings.AITimeoutSeconds {
		a.jevMu.Lock()
		a.jevRevision++
		for _, p := range a.aiPending {
			p.cancel()
		}
		if a.jevBad != nil {
			delete(a.jevBad, jev.KeySlot(v.JevEndpoint()))
		}
		delete(a.aiStopped, jev.KeySlot(v.JevEndpoint()))
		a.jevMu.Unlock()
		a.emitJev()
	}
	a.changed()
	return nil
}
func (a *App) GetShortcutStatus() string {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	return a.shortcutError
}
func (a *App) ToggleToday(id int64) error {
	if e := a.store.ToggleToday(id); e != nil {
		return e
	}
	a.changed()
	return nil
}
func (a *App) UndoCompletion(token string) error {
	if e := a.store.UndoCompletion(token); e != nil {
		return e
	}
	a.changed()
	return nil
}
func (a *App) showQuickAdd()      { a.showQuickAddMode(false) }
func (a *App) showQuickAtCursor() { a.showQuickAddMode(true) }
func (a *App) showQuickAddMode(cursor bool) {
	if a.quitting.Load() || a.tray == nil {
		return
	}
	a.hideQuickSuggestions()
	a.quickGeneration.Add(1)
	a.quickCursor.Store(cursor)
	a.captureQuickPoint()
	a.rememberQuickFocus()
	a.tray.ShowWindow()
	if cursor {
		a.placeQuickAtCursor(false)
	} else {
		a.clampQuickToWorkArea()
	}
	a.desktop.Event.Emit("board:mini-focus", a.quickGeneration.Load())
}
func (a *App) SetQuickAddExpanded(expanded bool) {
	a.SetQuickAddLayout(expanded, false, 0, a.quickGeneration.Load())
}
func quickAddHeight(options bool, rows int) int {
	if options {
		return 430 + min(max(rows, 0), 12)*26
	}
	return 105 + min(max(rows, 0), 12)*26
}
func (a *App) SetQuickAddLayout(expanded, options bool, rows int, generation uint64) {
	application.InvokeSync(func() {
		if generation != a.quickGeneration.Load() {
			return
		}
		if a.mini == nil {
			return
		}
		height := quickAddHeight(options, rows)
		_ = expanded // Suggestions never resize the input window.
		currentWidth, currentHeight := a.mini.Size()
		if currentWidth == 360 && currentHeight == height {
			return
		}
		a.hideQuickSuggestionWindow()
		a.mini.SetSize(360, height)
		if a.quickCursor.Load() {
			a.placeQuickAtCursor(false)
		} else if a.tray != nil {
			a.tray.PositionWindow(a.mini, 8)
			a.clampQuickToWorkArea()
		}
		a.refreshQuickSuggestions()
	})
}
func (a *App) Acknowledge(id int64) error {
	e := a.store.Acknowledge(id)
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) Snooze(id int64, minutes int) error {
	if minutes < 1 || minutes > 10080 {
		return errors.New("再通知は1分〜7日後で指定してください")
	}
	e := a.store.Snooze(id, board.ISO(time.Now().Add(time.Duration(minutes)*time.Minute)))
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) TestNotification() error {
	e := a.store.TestNotification()
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) OpenTask(id int64) { a.openMain(id) }
func (a *App) HideQuickAdd() {
	generation := a.quickGeneration.Add(1)
	a.desktop.Event.Emit("board:mini-reset", generation)
	if a.mini != nil {
		a.hideQuickNative(generation)
	}
}

func (a *App) ToggleImportant(id int64) error {
	if e := a.store.ToggleImportant(id); e != nil {
		return e
	}
	a.changed()
	return nil
}
func (a *App) OpenFromNotice(notificationID, taskID int64) error {
	if e := a.store.Acknowledge(notificationID); e != nil {
		return e
	}
	a.openMain(taskID)
	a.changed()
	return nil
}
func (a *App) OpenNotifications() { a.openMain(0); a.desktop.Event.Emit("board:notices") }
func (a *App) MoveTask(id, categoryID int64) error {
	e := a.store.MoveTask(id, categoryID)
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) ReorderCategories(ids []int64) error {
	e := a.store.ReorderCategories(ids)
	if e == nil {
		a.changed()
	}
	return e
}
func (a *App) SaveMainWindowSize() error { w, h := a.main.Size(); return a.store.SaveWindowSize(w, h) }
func (a *App) FinishClose(mode string) {
	if e := a.saveMainWindowBounds(); e != nil {
		a.desktop.Event.Emit("board:error", e.Error())
		return
	}
	if e := a.SaveMainWindowSize(); e != nil {
		a.desktop.Event.Emit("board:error", e.Error())
		return
	}
	if mode == "quit" {
		a.quitting.Store(true)
		a.stopOnce.Do(func() { close(a.stop) })
		a.wg.Wait()
		a.desktop.Quit()
	} else {
		a.main.Hide()
	}
}
func (a *App) SaveImage(dataURL string) (string, error) {
	_, encoded, ok := strings.Cut(dataURL, ",")
	if !ok || len(encoded) > 28*1024*1024 {
		return "", errors.New("画像は20MB以内にしてください")
	}
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return "", e
	}
	if len(b) > 20<<20 {
		return "", errors.New("画像は20MB以内にしてください")
	}
	mime := http.DetectContentType(b)
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp"}[mime]
	if ext == "" {
		return "", errors.New("PNG・JPEG・GIF・WebP画像を指定してください")
	}
	name := uuid.NewString() + ext
	if e = os.WriteFile(filepath.Join(a.store.Dir, "images", name), b, 0644); e != nil {
		return "", e
	}
	return "/images/" + name, nil
}
func (a *App) OpenURL(target string) error {
	u, e := url.Parse(target)
	if e != nil {
		return e
	}
	if (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
		return a.desktop.Browser.OpenURL(target)
	}
	if u.Scheme == "file" {
		path := u.Path
		if u.Host != "" && u.Host != "localhost" {
			path = "//" + u.Host + path
		}
		if runtime.GOOS == "windows" {
			path = strings.ReplaceAll(path, "/", "\\")
			if len(path) > 3 && path[0] == '\\' && path[2] == ':' {
				path = path[1:]
			}
		}
		if !filepath.IsAbs(path) {
			return errors.New("絶対パスを指定してください")
		}
		return openFile(path)
	}
	return errors.New("HTTP/HTTPSまたはファイルリンクを指定してください")
}
func (a *App) OpenDataFolder() error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", a.store.Dir)
	case "darwin":
		cmd = exec.Command("open", a.store.Dir)
	default:
		cmd = exec.Command("xdg-open", a.store.Dir)
	}
	return cmd.Start()
}
func (a *App) Backup() (string, error) { return a.store.Backup() }

// The legacy byte API remains available for tests and existing clients, but the
// desktop UI uses a native picker and streams the selected ZIP from disk.
func (a *App) RestoreBackup(encoded string) error {
	data, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return e
	}
	return a.restoreBackup(func() error { return a.store.Restore(data) })
}
func (a *App) RestoreBackupFile() (bool, error) {
	path, e := a.desktop.Dialog.OpenFile().SetTitle("Kotorimaのバックアップを選択").AddFilter("バックアップZIP", "*.zip").PromptForSingleSelection()
	if e != nil || path == "" {
		return false, e
	}
	e = a.restoreBackup(func() error { return a.store.RestoreFile(path) })
	return e == nil, e
}
func (a *App) restoreBackup(restore func() error) error {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if _, e := a.store.Backup(); e != nil {
		return fmt.Errorf("復元前の退避に失敗しました: %w", e)
	}
	if e := restore(); e != nil {
		return e
	}
	a.mu.Lock()
	a.seen = map[int64]bool{}
	a.mu.Unlock()
	if v, e := a.store.Snapshot(); e == nil && a.shortcut != nil {
		if e = a.shortcut.Change(v.Settings.QuickShortcut); e != nil {
			a.shortcut.Change("")
			a.shortcutError = e.Error()
			a.desktop.Event.Emit("board:error", a.shortcutError)
		} else {
			a.shortcutError = ""
		}
	}
	a.changed()
	return nil
}

func (a *App) traySingleClick() {
	wait := doubleClickDelay()
	if last := a.trayDouble.Load(); last != 0 && time.Since(time.Unix(0, last)) < wait {
		return
	}
	generation := a.trayGeneration.Add(1)
	time.AfterFunc(wait, func() {
		if a.quitting.Load() || a.trayGeneration.Load() != generation {
			return
		}
		a.showQuickAdd()
	})
}
func (a *App) trayDoubleClick() {
	a.trayDouble.Store(time.Now().UnixNano())
	a.trayGeneration.Add(1)
	a.mini.Hide()
	a.openMain(0)
}
