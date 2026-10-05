//go:build nativesmoke && windows && !server

package main

import (
	"encoding/json"
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
	"kotorima/internal/board"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"time"
	"unsafe"
)

const probeTitle = "Kotorima focus probe"

func runNativeProbe() bool {
	if len(os.Args) < 2 || os.Args[1] != "--focus-probe" {
		return false
	}
	runtime.LockOSThread()
	class, _ := windows.UTF16PtrFromString("STATIC")
	title, _ := windows.UTF16PtrFromString(probeTitle)
	h, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(title)), 0x10cf0000, 100, 100, 300, 180, 0, 0, 0, 0)
	user32.NewProc("SetForegroundWindow").Call(h)
	type msg struct {
		Window         uintptr
		Message        uint32
		WParam, LParam uintptr
		Time           uint32
		X, Y           int32
		Private        uint32
	}
	var m msg
	for {
		r, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if r == 0 || r == ^uintptr(0) {
			break
		}
		user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
		user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
	}
	return true
}

var smokePoint = make(chan [2]float64, 1)

func (a *App) SmokeTarget(x, y float64) {
	select {
	case smokePoint <- [2]float64{x, y}:
	default:
	}
}
func checkNoticeRectangles(a *App) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	windows := []*application.WebviewWindow{a.notice}
	for _, w := range a.noticeWindows {
		windows = append(windows, w)
	}
	type rect struct{ Left, Top, Right, Bottom int32 }
	rects := []rect{}
	for _, w := range windows {
		hwnd := uintptr(w.NativeWindow())
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		if visible == 0 {
			continue
		}
		var r rect
		getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
		for _, other := range rects {
			if r.Top < other.Bottom && other.Top < r.Bottom && r.Left < other.Right && other.Left < r.Right {
				return fmt.Errorf("visible notifications overlap: %v and %v", r, other)
			}
		}
		rects = append(rects, r)
	}
	if len(rects) < 2 {
		return fmt.Errorf("expected multiple visible notification rectangles, got %d", len(rects))
	}
	return nil
}

func checkRightEdgeResize(a *App) error {
	for i := 0; i < 30; i++ {
		if _, e := a.store.SaveTask(board.Task{Title: fmt.Sprintf("Resize probe %d", i)}); e != nil {
			return e
		}
	}
	a.desktop.Event.Emit("board:changed")
	time.Sleep(800 * time.Millisecond)
	a.main.ExecJS(`(()=>{const el=document.querySelector('.board-main');if(el)fetch('/wails/runtime?object=0&method=0&args='+encodeURIComponent(JSON.stringify({'call-id':'native-scroll-probe',methodName:'main.App.SmokeTarget',args:[el.scrollHeight,el.clientHeight]})))})()`)
	select {
	case p := <-smokePoint:
		if p[0] <= p[1] {
			return fmt.Errorf("resize probe did not produce a scrollbar")
		}
	case <-time.After(5 * time.Second):
		return fmt.Errorf("resize probe renderer did not respond")
	}
	original := a.main.Bounds()
	defer a.main.SetBounds(original)
	var before, after struct{ Left, Top, Right, Bottom int32 }
	hwnd := uintptr(a.main.NativeWindow())
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&before)))
	x, y := before.Right-3, (before.Top+before.Bottom)/2
	user32.NewProc("SetCursorPos").Call(uintptr(x), uintptr(y))
	time.Sleep(100 * time.Millisecond)
	user32.NewProc("mouse_event").Call(2, 0, 0, 0, 0)
	user32.NewProc("SetCursorPos").Call(uintptr(x+8), uintptr(y))
	time.Sleep(100 * time.Millisecond)
	user32.NewProc("SetCursorPos").Call(uintptr(x+50), uintptr(y))
	time.Sleep(100 * time.Millisecond)
	user32.NewProc("mouse_event").Call(4, 0, 0, 0, 0)
	time.Sleep(200 * time.Millisecond)
	getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&after)))
	if after.Right-after.Left < before.Right-before.Left+20 {
		return fmt.Errorf("right window edge did not resize with scrollbar: before=%v after=%v", before, after)
	}
	return nil
}

func checkMainSuggestionsRestore(a *App) error {
	for n := 0; n < 5; n++ {
		for i := 0; i < 3; i++ {
			if _, e := a.store.SaveTask(board.Task{Title: fmt.Sprintf("Restore candidate %d", n)}); e != nil {
				return e
			}
		}
	}
	a.desktop.Event.Emit("board:changed")
	a.main.Show()
	a.main.Focus()
	time.Sleep(400 * time.Millisecond)
	a.main.ExecJS(`(()=>{const el=document.querySelector('textarea[aria-label="新しい付箋"]');el?.focus();el?.click()})()`)
	check := func(want bool) error {
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			a.main.ExecJS(`fetch('/wails/runtime?object=0&method=0&args='+encodeURIComponent(JSON.stringify({'call-id':'native-main-suggestions',methodName:'main.App.SmokeTarget',args:[document.querySelectorAll('.quick .task-suggestions button').length,0]})))`)
			select {
			case p := <-smokePoint:
				if (p[0] > 0) == want {
					return nil
				}
			case <-time.After(time.Until(until)):
				return fmt.Errorf("main suggestion renderer timed out")
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("unexpected main suggestions visibility, expected %v", want)
	}
	if e := check(true); e != nil {
		return e
	}
	a.main.Hide()
	time.Sleep(150 * time.Millisecond)
	application.InvokeSync(func() { a.trayDoubleClick() })
	time.Sleep(250 * time.Millisecond)
	a.main.ExecJS(`document.querySelector('textarea[aria-label="新しい付箋"]')?.focus()`)
	if e := check(false); e != nil {
		return fmt.Errorf("tray double click reopened main suggestions: %w", e)
	}
	return nil
}

// Exercise the actual owned, non-activating dropdown near the taskbar edge.
func checkQuickDropdown(a *App, foreground uintptr) error {
	a.showQuickAtCursor()
	time.Sleep(250 * time.Millisecond)
	h := uintptr(a.mini.NativeWindow())
	monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(h, 2)
	var info struct {
		Size          uint32
		Monitor, Work struct{ Left, Top, Right, Bottom int32 }
		Flags         uint32
	}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		return fmt.Errorf("quick dropdown monitor unavailable")
	}
	var before, after struct{ Left, Top, Right, Bottom int32 }
	getWindowRect.Call(h, uintptr(unsafe.Pointer(&before)))
	height := before.Bottom - before.Top
	application.InvokeSync(func() {
		a.mini.SetBounds(a.desktop.Screen.PhysicalToDipRect(application.Rect{X: int(info.Work.Left + 20), Y: int(info.Work.Bottom - height - 8), Width: int(before.Right - before.Left), Height: int(height)}))
	})
	getWindowRect.Call(h, uintptr(unsafe.Pointer(&before)))
	a.SetQuickSuggestions(QuickSuggestions{Items: []string{"Native suggestion", "Second", "Third", "Fourth", "Fifth"}, Index: 0, Revision: 1000000, Generation: a.quickGeneration.Load(), Anchor: application.Rect{X: 12, Y: 35, Width: 334, Height: 36}})
	deadline := time.Now().Add(5 * time.Second)
	visible := uintptr(0)
	for time.Now().Before(deadline) {
		visible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(a.quickSuggestions.NativeWindow()))
		if visible != 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if visible == 0 {
		return fmt.Errorf("quick suggestion popup did not appear")
	}
	getWindowRect.Call(h, uintptr(unsafe.Pointer(&after)))
	if before != after {
		return fmt.Errorf("suggestions resized input: %v -> %v", before, after)
	}
	miniDPI, _, _ := user32.NewProc("GetDpiForWindow").Call(h)
	var popup struct{ Left, Top, Right, Bottom int32 }
	getWindowRect.Call(uintptr(a.quickSuggestions.NativeWindow()), uintptr(unsafe.Pointer(&popup)))
	if popup.Left < info.Work.Left || popup.Right > info.Work.Right || popup.Top < info.Work.Top || popup.Bottom > info.Work.Bottom || popup.Bottom > before.Top+int32(float64(35)*float64(miniDPI)/96) {
		return fmt.Errorf("suggestions did not fit above input and outside taskbar: input=%v popup=%v work=%v", before, popup, info.Work)
	}
	active, _, _ := getForegroundWindow.Call()
	if active != h {
		return fmt.Errorf("suggestions stole input focus")
	}
	a.quickSuggestions.ExecJS(`(()=>{const until=Date.now()+5000;const probe=()=>{const b=document.querySelector('button');if(!b){if(Date.now()<until)setTimeout(probe,50);return;}const r=b.getBoundingClientRect();fetch('/wails/runtime?object=0&method=0&args='+encodeURIComponent(JSON.stringify({'call-id':'native-suggestion-click',methodName:'main.App.SmokeTarget',args:[r.x+r.width/2,r.y+r.height/2]})))};probe()})()`)
	var p [2]float64
	select {
	case p = <-smokePoint:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("suggestion renderer not ready: %+v", a.GetQuickSuggestions())
	}
	ph := uintptr(a.quickSuggestions.NativeWindow())
	dpi, _, _ := user32.NewProc("GetDpiForWindow").Call(ph)
	point := struct{ X, Y int32 }{int32(p[0] * float64(dpi) / 96), int32(p[1] * float64(dpi) / 96)}
	user32.NewProc("ClientToScreen").Call(ph, uintptr(unsafe.Pointer(&point)))
	user32.NewProc("SetCursorPos").Call(uintptr(point.X), uintptr(point.Y))
	user32.NewProc("mouse_event").Call(2, 0, 0, 0, 0)
	user32.NewProc("mouse_event").Call(4, 0, 0, 0, 0)
	checkText := func(expected string) error {
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			a.mini.ExecJS(fmt.Sprintf(`(()=>{const v=document.querySelector('input[aria-label="トレイからタスク追加"]')?.value;fetch('/wails/runtime?object=0&method=0&args='+encodeURIComponent(JSON.stringify({'call-id':'native-draft-probe',methodName:'main.App.SmokeTarget',args:[v===%q?1:0,0]})))})()`, expected))
			select {
			case value := <-smokePoint:
				if value[0] == 1 {
					return nil
				}
			case <-time.After(time.Until(until)):
				return fmt.Errorf("quick input draft probe timed out")
			}
			time.Sleep(50 * time.Millisecond)
		}
		return fmt.Errorf("unexpected quick input draft, expected %q", expected)
	}
	if e := checkText("Native suggestion"); e != nil {
		return e
	}
	user32.NewProc("SetForegroundWindow").Call(foreground)
	time.Sleep(300 * time.Millisecond)
	a.showQuickAtCursor()
	if e := checkText("Native suggestion"); e != nil {
		return fmt.Errorf("focus-loss draft: %w", e)
	}
	a.mini.ExecJS(`document.querySelector('button[aria-label="入力欄を閉じる"]')?.click()`)
	if e := checkText(""); e != nil {
		return fmt.Errorf("cancel did not discard input: %w", e)
	}
	time.Sleep(150 * time.Millisecond)
	a.showQuickAtCursor()
	if e := checkText(""); e != nil {
		return fmt.Errorf("explicit cancel draft: %w", e)
	}
	a.HideQuickAdd()
	return nil
}

func clickAcknowledge(a *App) error {
	a.notice.ExecJS(`(()=>{const b=document.querySelector('.notice-window header button[aria-label="通知を閉じる"]');if(!b)return;const r=b.getBoundingClientRect();fetch('/wails/runtime?object=0&method=0&args='+encodeURIComponent(JSON.stringify({'call-id':'native-click',methodName:'main.App.SmokeTarget',args:[r.x+r.width/2,r.y+r.height/2]})))})()`)
	var p [2]float64
	select {
	case p = <-smokePoint:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("notification renderer did not expose acknowledgement button")
	}
	h := uintptr(a.notice.NativeWindow())
	dpi, _, _ := user32.NewProc("GetDpiForWindow").Call(h)
	point := struct{ X, Y int32 }{int32(p[0] * float64(dpi) / 96), int32(p[1] * float64(dpi) / 96)}
	user32.NewProc("ClientToScreen").Call(h, uintptr(unsafe.Pointer(&point)))
	user32.NewProc("SetCursorPos").Call(uintptr(point.X), uintptr(point.Y))
	user32.NewProc("mouse_event").Call(2, 0, 0, 0, 0)
	user32.NewProc("mouse_event").Call(4, 0, 0, 0, 0)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, e := a.store.Snapshot()
		if e != nil {
			return e
		}
		pending := false
		for _, n := range v.Notifications {
			pending = pending || !n.Acknowledged
		}
		if !pending {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("notification acknowledgement did not respond to physical mouse click")
}
func checkImageViewer(a *App) error {
	src, err := a.SaveImage("data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aCfkAAAAASUVORK5CYII=")
	if err != nil {
		return err
	}
	if err = a.OpenImageViewer(src); err != nil {
		return err
	}
	defer a.CloseImageViewer()
	until := time.Now().Add(10 * time.Second)
	loaded := false
	for time.Now().Before(until) {
		a.imageViewer.ExecJS(`(()=>{const img=document.querySelector('.image-viewer img');fetch('/wails/runtime?object=0&method=0&args='+encodeURIComponent(JSON.stringify({'call-id':'image-viewer-probe',methodName:'main.App.SmokeTarget',args:[img?.complete&&img.naturalWidth>0?1:0,innerWidth]})))})()`)
		select {
		case p := <-smokePoint:
			if p[0] == 1 && p[1] >= 360 {
				loaded = true
			}
		case <-time.After(time.Until(until)):
			return fmt.Errorf("image viewer renderer timed out")
		}
		if loaded {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !loaded {
		return fmt.Errorf("image viewer did not load the saved image")
	}
	hwnd := uintptr(a.imageViewer.NativeWindow())
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
	if visible == 0 {
		return fmt.Errorf("image viewer remained hidden")
	}
	a.imageViewer.ExecJS(`document.querySelector('button[aria-label="画像ビューアを閉じる"]')?.click()`)
	until = time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		visible, _, _ = user32.NewProc("IsWindowVisible").Call(hwnd)
		if visible == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if visible != 0 {
		return fmt.Errorf("image viewer close button did not hide window")
	}
	if err = a.OpenImageViewer(src); err != nil {
		return err
	}
	visible, _, _ = user32.NewProc("IsWindowVisible").Call(hwnd)
	if visible == 0 {
		return fmt.Errorf("image viewer could not reopen")
	}
	return nil
}

func startNativeVerification(a *App) {
	go func() {
		result := map[string]any{"passed": false}
		finish := func(e error) {
			if e != nil {
				result["error"] = e.Error()
			} else {
				result["passed"] = true
			}
			b, _ := json.MarshalIndent(result, "", "  ")
			os.WriteFile("native-smoke.json", b, 0644)
			go func() {
				time.Sleep(10 * time.Second)
				f, _ := os.Create("native-shutdown.txt")
				if f != nil {
					pprof.Lookup("goroutine").WriteTo(f, 2)
					f.Close()
				}
				os.Exit(2)
			}()
			a.FinishClose("quit")
		}
		time.Sleep(2 * time.Second)
		deadline := time.Now().Add(20 * time.Second)
		for !a.noticeReady.Load() && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if !a.noticeReady.Load() {
			finish(fmt.Errorf("notification renderer did not initialise"))
			return
		}
		if e := checkRightEdgeResize(a); e != nil {
			finish(e)
			return
		}
		result["right-edge-resize-with-scrollbar"] = true
		if e := checkImageViewer(a); e != nil {
			finish(e)
			return
		}
		result["image-viewer-load-close-reopen"] = true
		a.main.Hide()
		exe, _ := os.Executable()
		probe := exec.Command(exe, "--focus-probe")
		if e := probe.Start(); e != nil {
			finish(e)
			return
		}
		defer probe.Process.Kill()
		title, _ := windows.UTF16PtrFromString(probeTitle)
		var foreground uintptr
		deadline = time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			foreground, _, _ = user32.NewProc("FindWindowW").Call(0, uintptr(unsafe.Pointer(title)))
			active, _, _ := getForegroundWindow.Call()
			if foreground != 0 && active == foreground {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		active, _, _ := getForegroundWindow.Call()
		if foreground == 0 || active != foreground {
			finish(fmt.Errorf("could not establish external foreground window"))
			return
		}
		settings, _ := a.store.Snapshot()
		settings.Settings.Sound = false
		settings.Settings.NotifyTimes = []string{}
		a.store.SaveSettings(settings.Settings)
		check := func(label string) error {
			if e := a.TestNotification(); e != nil {
				return e
			}
			time.Sleep(time.Second)
			h := uintptr(a.notice.NativeWindow())
			visible, _, _ := user32.NewProc("IsWindowVisible").Call(h)
			style, _, _ := user32.NewProc("GetWindowLongPtrW").Call(h, ^uintptr(19))
			active, _, _ := getForegroundWindow.Call()
			if visible == 0 {
				return fmt.Errorf("%s: notification not visible", label)
			}
			if style&0x8 == 0 || style&0x80 != 0 || style&0x40000 == 0 {
				return fmt.Errorf("%s: incorrect topmost/taskbar style: %x", label, style)
			}
			if active != foreground {
				return fmt.Errorf("%s: keyboard focus stolen (before=%x after=%x)", label, foreground, active)
			}
			result[label] = true
			return nil
		}
		if e := check("hidden-main"); e != nil {
			finish(e)
			return
		}
		if e := a.TestNotification(); e != nil {
			finish(e)
			return
		}
		v, _ := a.store.Snapshot()
		pending := 0
		for _, n := range v.Notifications {
			if !n.Acknowledged {
				pending++
			}
		}
		if pending != 2 {
			finish(fmt.Errorf("expected two persistent notifications, got %d", pending))
			return
		}
		time.Sleep(2 * time.Second)
		a.mu.Lock()
		secondary := len(a.noticeWindows)
		var extra *application.WebviewWindow
		for _, w := range a.noticeWindows {
			extra = w
		}
		a.mu.Unlock()
		if secondary != 1 || extra == nil {
			finish(fmt.Errorf("second notification has no independent window"))
			return
		}
		var secondVisible uintptr
		deadline = time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			secondVisible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(extra.NativeWindow()))
			active, _, _ = getForegroundWindow.Call()
			if active != foreground {
				finish(fmt.Errorf("second notification stole focus (before=%x after=%x visible=%d)", foreground, active, secondVisible))
				return
			}
			if secondVisible != 0 {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if secondVisible == 0 {
			a.mu.Lock()
			loaded := a.noticeLoaded
			shown := a.noticeShown
			message := fmt.Errorf("second notification remained hidden (loaded=%v shown=%v hwnd=%x)", loaded, shown, uintptr(extra.NativeWindow()))
			a.mu.Unlock()
			finish(message)
			return
		}
		var firstRect, secondRect struct{ Left, Top, Right, Bottom int32 }
		getWindowRect.Call(uintptr(a.notice.NativeWindow()), uintptr(unsafe.Pointer(&firstRect)))
		getWindowRect.Call(uintptr(extra.NativeWindow()), uintptr(unsafe.Pointer(&secondRect)))
		if firstRect.Top < secondRect.Bottom && secondRect.Top < firstRect.Bottom && firstRect.Left < secondRect.Right && secondRect.Left < firstRect.Right {
			finish(fmt.Errorf("notification windows overlap"))
			return
		}
		result["multiple-notifications"] = true
		time.Sleep(2 * time.Second)
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(uintptr(a.notice.NativeWindow()))
		if visible == 0 {
			finish(fmt.Errorf("notification disappeared without acknowledgement"))
			return
		}
		result["persistent-until-ack"] = true
		// Mixed-size notifications and promotion after dismissing the first.
		if _, e := a.store.SaveTask(board.Task{Title: "Summary placement probe", Deadline: time.Now().Format("2006-01-02")}); e != nil {
			finish(e)
			return
		}
		prefs, _ := a.store.Snapshot()
		prefs.Settings.NotifyTimes = []string{time.Now().Format("15:04")}
		prefs.Settings.NotifyWeekdays = []int{int(time.Now().Weekday())}
		if e := a.store.SaveSettings(prefs.Settings); e != nil {
			finish(e)
			return
		}
		if _, e := a.store.Tick(time.Now(), false); e != nil {
			finish(e)
			return
		}
		a.refreshNotice()
		time.Sleep(2 * time.Second)
		if e := checkNoticeRectangles(a); e != nil {
			finish(e)
			return
		}
		result["mixed-size-notifications"] = true
		snapshot, _ := a.store.Snapshot()
		for _, n := range snapshot.Notifications {
			if !n.Acknowledged {
				a.Acknowledge(n.ID)
				break
			}
		}
		time.Sleep(500 * time.Millisecond)
		if e := checkNoticeRectangles(a); e != nil {
			finish(e)
			return
		}
		result["notification-promotion-layout"] = true
		prefs.Settings.NotifyTimes = []string{}
		a.store.SaveSettings(prefs.Settings)
		a.Acknowledge(0)
		visible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(a.notice.NativeWindow()))
		if visible != 0 {
			finish(fmt.Errorf("acknowledged window stayed visible"))
			return
		}
		result["ack-hides-window"] = true
		a.main.Show()
		a.main.Minimise()
		user32.NewProc("SetForegroundWindow").Call(foreground)
		time.Sleep(500 * time.Millisecond)
		if e := check("minimised-main"); e != nil {
			finish(e)
			return
		}
		application.InvokeSync(func() { setWindowPos.Call(uintptr(a.notice.NativeWindow()), ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x10) })
		if e := clickAcknowledge(a); e != nil {
			finish(e)
			return
		}
		result["mouse-acknowledgement"] = true
		a.main.Hide()
		application.InvokeSync(a.traySingleClick)
		time.Sleep(doubleClickDelay() + 500*time.Millisecond)
		miniVisible, _, _ := user32.NewProc("IsWindowVisible").Call(uintptr(a.mini.NativeWindow()))
		mainVisible, _, _ := user32.NewProc("IsWindowVisible").Call(uintptr(a.main.NativeWindow()))
		if miniVisible == 0 || mainVisible != 0 {
			finish(fmt.Errorf("tray single click did not open only the quick input"))
			return
		}
		a.mini.Hide()
		a.trayDouble.Store(0)
		application.InvokeSync(func() { a.traySingleClick(); a.trayDoubleClick(); a.traySingleClick() })
		time.Sleep(doubleClickDelay() + 500*time.Millisecond)
		miniVisible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(a.mini.NativeWindow()))
		mainVisible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(a.main.NativeWindow()))
		if miniVisible != 0 || mainVisible == 0 {
			finish(fmt.Errorf("tray double click left a delayed quick input"))
			return
		}
		if err := checkMainSuggestionsRestore(a); err != nil {
			finish(err)
			return
		}
		result["main-suggestions-not-reopened-by-tray-double-click"] = true
		// Exercise real RegisterHotKey delivery and atomic conflict handling.
		settings, err := a.store.Snapshot()
		if err != nil {
			finish(err)
			return
		}
		savedShortcut := settings.Settings.QuickShortcut
		settings.Settings.QuickShortcut = "Ctrl+Alt+Shift+F11"
		if err = a.SaveSettings(settings.Settings); err != nil {
			finish(err)
			return
		}
		competitor := newShortcutManager(func() {})
		if err = competitor.Change(settings.Settings.QuickShortcut); err == nil {
			competitor.Close()
			finish(fmt.Errorf("duplicate global shortcut accepted"))
			return
		}
		settings.Settings.QuickShortcut = ""
		if err = a.SaveSettings(settings.Settings); err != nil {
			competitor.Close()
			finish(err)
			return
		}
		if err = competitor.Change("Ctrl+Alt+Shift+F11"); err != nil {
			competitor.Close()
			finish(err)
			return
		}
		settings.Settings.QuickShortcut = "Ctrl+Alt+Shift+F11"
		if err = a.SaveSettings(settings.Settings); err == nil {
			competitor.Close()
			finish(fmt.Errorf("conflicting shortcut was saved"))
			return
		}
		latest, _ := a.store.Snapshot()
		if latest.Settings.QuickShortcut != "" {
			competitor.Close()
			finish(fmt.Errorf("failed shortcut change modified persisted settings"))
			return
		}
		// A startup conflict must not block unrelated settings such as theme.
		if err = a.store.SaveSettings(settings.Settings); err != nil {
			competitor.Close()
			finish(err)
			return
		}
		a.settingsMu.Lock()
		a.shortcutError = "startup conflict"
		a.settingsMu.Unlock()
		unrelated := settings.Settings
		unrelated.Theme = "dark"
		if err = a.SaveSettings(unrelated); err != nil {
			competitor.Close()
			finish(fmt.Errorf("startup shortcut conflict blocked theme setting: %w", err))
			return
		}
		if a.GetShortcutStatus() == "" {
			competitor.Close()
			finish(fmt.Errorf("unrelated save cleared shortcut warning"))
			return
		}
		unrelated.QuickShortcut = ""
		if err = a.SaveSettings(unrelated); err != nil {
			competitor.Close()
			finish(err)
			return
		}
		competitor.Close()
		// General keys use the same native registration and conflict safeguards.
		general := settings.Settings
		general.QuickShortcut = "Ctrl+Alt+Shift+Space"
		if err = a.SaveSettings(general); err != nil {
			finish(err)
			return
		}
		spaceCompetitor := newShortcutManager(func() {})
		if err = spaceCompetitor.Change(general.QuickShortcut); err == nil {
			spaceCompetitor.Close()
			finish(fmt.Errorf("duplicate Space shortcut accepted"))
			return
		}
		spaceCompetitor.Close()
		result["global-shortcut-space"] = true
		if err = a.SaveSettings(settings.Settings); err != nil {
			finish(err)
			return
		}
		a.main.Hide()
		a.mini.Hide()
		user32.NewProc("SetForegroundWindow").Call(foreground)
		user32.NewProc("SetCursorPos").Call(220, 170)
		keyboard := user32.NewProc("keybd_event")
		for _, key := range []uintptr{0x11, 0x12, 0x10, 0x7A} {
			keyboard.Call(key, 0, 0, 0)
		}
		for _, key := range []uintptr{0x7A, 0x10, 0x12, 0x11} {
			keyboard.Call(key, 0, 2, 0)
		}
		deadline = time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			miniVisible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(a.mini.NativeWindow()))
			active, _, _ = getForegroundWindow.Call()
			if miniVisible != 0 && active == uintptr(a.mini.NativeWindow()) {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if miniVisible == 0 || active != uintptr(a.mini.NativeWindow()) {
			finish(fmt.Errorf("global shortcut did not focus quick input"))
			return
		}
		var quickRect struct{ Left, Top, Right, Bottom int32 }
		user32.NewProc("GetWindowRect").Call(uintptr(a.mini.NativeWindow()), uintptr(unsafe.Pointer(&quickRect)))
		if quickRect.Left != 232 || quickRect.Top != 182 {
			finish(fmt.Errorf("quick input not at cursor: %+v", quickRect))
			return
		}
		result["quick-input-at-cursor"] = true
		previous := a.quickPrevious.Load()
		a.HideQuickAdd()
		time.Sleep(200 * time.Millisecond)
		active, _, _ = getForegroundWindow.Call()
		if active != foreground {
			finish(fmt.Errorf("closing quick input did not return to original app: expected=%x saved=%x active=%x", foreground, previous, active))
			return
		}
		a.showQuickAtCursor()
		time.Sleep(200 * time.Millisecond)
		user32.NewProc("SetForegroundWindow").Call(foreground)
		deadline = time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			miniVisible, _, _ = user32.NewProc("IsWindowVisible").Call(uintptr(a.mini.NativeWindow()))
			if miniVisible == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if miniVisible != 0 {
			finish(fmt.Errorf("quick input did not hide after focus loss"))
			return
		}
		active, _, _ = getForegroundWindow.Call()
		if active != foreground {
			finish(fmt.Errorf("auto-hide stole focus"))
			return
		}
		result["quick-input-blur-hides-without-focus-theft"] = true
		if err = checkQuickDropdown(a, foreground); err != nil {
			finish(err)
			return
		}
		result["quick-suggestions-placement-focus-draft-cancel"] = true
		settings.Settings.QuickShortcut = savedShortcut
		if err = a.SaveSettings(settings.Settings); err != nil {
			finish(err)
			return
		}
		result["global-shortcut-change-disable-conflict-focus"] = true
		result["tray-single-double-click"] = true
		finish(nil)
	}()
}
