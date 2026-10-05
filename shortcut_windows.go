//go:build windows && !server

package main

import (
	"fmt"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
	"kotorima/internal/board"
	"runtime"
	"sync"
	"time"
	"unsafe"
)

type shortcutCommand struct {
	key   string
	stop  bool
	reply chan error
}
type shortcutManager struct {
	mu       sync.Mutex
	thread   uint32
	commands chan shortcutCommand
	done     chan struct{}
	closed   bool
}
type shortcutMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	X, Y           int32
	Private        uint32
}

func newShortcutManager(callback func()) *shortcutManager {
	m := &shortcutManager{commands: make(chan shortcutCommand, 1), done: make(chan struct{})}
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(m.done)
		var msg shortcutMessage
		// Ensure the thread's message queue exists before exposing its ID.
		user32.NewProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
		m.thread = windows.GetCurrentThreadId()
		close(ready)
		key := ""
		id := uintptr(1)
		registered := false
		defer func() {
			if registered {
				user32.NewProc("UnregisterHotKey").Call(0, id)
			}
		}()
		for {
			r, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if r == 0 || r == ^uintptr(0) {
				return
			}
			if msg.Message == 0x0312 && registered && msg.WParam == id {
				go callback()
			}
			if msg.Message != 0x8001 {
				continue
			}
			command := <-m.commands
			if command.stop {
				command.reply <- nil
				return
			}
			if command.key == key {
				command.reply <- nil
				continue
			}
			mods, vk, e := board.ParseShortcut(command.key)
			if e == nil && command.key != "" {
				nextID := uintptr(3) - id
				ok, _, _ := user32.NewProc("RegisterHotKey").Call(0, nextID, uintptr(mods)|0x4000, uintptr(vk))
				if ok == 0 {
					e = fmt.Errorf("%s は他のアプリまたはWindowsで使用されています。別のキーを選んでください", command.key)
				} else {
					if registered {
						user32.NewProc("UnregisterHotKey").Call(0, id)
					}
					id = nextID
					registered = true
					key = command.key
				}
			} else if e == nil {
				if registered {
					user32.NewProc("UnregisterHotKey").Call(0, id)
				}
				registered = false
				key = ""
			}
			command.reply <- e
		}
	}()
	<-ready
	return m
}
func (m *shortcutManager) Change(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("アプリを終了中です")
	}
	return m.send(shortcutCommand{key: key, reply: make(chan error, 1)})
}
func (m *shortcutManager) send(command shortcutCommand) error {
	select {
	case <-m.done:
		return fmt.Errorf("ショートカット処理が停止しています")
	default:
	}
	m.commands <- command
	ok, _, err := user32.NewProc("PostThreadMessageW").Call(uintptr(m.thread), 0x8001, 0, 0)
	if ok == 0 {
		<-m.commands
		return err
	}
	select {
	case e := <-command.reply:
		return e
	case <-m.done:
		return fmt.Errorf("ショートカット処理が停止しました")
	}
}
func (m *shortcutManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.closed = true
	m.send(shortcutCommand{stop: true, reply: make(chan error, 1)})
	<-m.done
}
func (a *App) rememberQuickFocus() {
	foreground, _, _ := getForegroundWindow.Call()
	if foreground != uintptr(a.mini.NativeWindow()) {
		a.quickPrevious.Store(foreground)
	}
}
func (a *App) hideQuickNative(generation uint64) {
	application.InvokeSync(func() {
		if generation != a.quickGeneration.Load() {
			return
		}
		previous := a.quickPrevious.Swap(0)
		foreground, _, _ := getForegroundWindow.Call()
		wasQuick := foreground == uintptr(a.mini.NativeWindow())
		// Hiding an active window can select another window automatically. Return
		// focus afterwards, on the UI thread; never override a user's app switch.
		a.hideQuickSuggestions()
		a.mini.Hide()
		if !wasQuick || previous == 0 {
			return
		}
		valid, _, _ := user32.NewProc("IsWindow").Call(previous)
		if valid == 0 {
			return
		}
		user32.NewProc("SetForegroundWindow").Call(previous)
	})
}

// Coordinates come from Win32 in physical pixels, as do the native bounds.
func (a *App) placeQuickAtCursor(capture bool) {
	application.InvokeSync(func() {
		if capture {
			var point struct{ X, Y int32 }
			ok, _, _ := user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
			if ok == 0 {
				return
			}
			a.quickX, a.quickY = point.X, point.Y
		}
		packed := uintptr(uint64(uint32(a.quickX)) | uint64(uint32(a.quickY))<<32)
		monitor, _, _ := user32.NewProc("MonitorFromPoint").Call(packed, 2)
		var info struct {
			Size          uint32
			Monitor, Work struct{ Left, Top, Right, Bottom int32 }
			Flags         uint32
		}
		info.Size = uint32(unsafe.Sizeof(info))
		ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info)))
		if ok == 0 {
			return
		}
		var rect struct{ Left, Top, Right, Bottom int32 }
		ok, _, _ = user32.NewProc("GetWindowRect").Call(uintptr(a.mini.NativeWindow()), uintptr(unsafe.Pointer(&rect)))
		if ok == 0 {
			return
		}
		x, y := a.quickX+12, a.quickY+12
		width, height := rect.Right-rect.Left, rect.Bottom-rect.Top
		if x+width > info.Work.Right {
			x = info.Work.Right - width
		}
		if y+height > info.Work.Bottom {
			y = info.Work.Bottom - height
		}
		if x < info.Work.Left {
			x = info.Work.Left
		}
		if y < info.Work.Top {
			y = info.Work.Top
		}
		user32.NewProc("SetWindowPos").Call(uintptr(a.mini.NativeWindow()), 0, uintptr(x), uintptr(y), 0, 0, 0x15)
	})
}

// A manually moved mini window keeps its position while its options expand.
func (a *App) rememberQuickPosition() {
	application.InvokeSync(func() {
		var r struct{ Left, Top, Right, Bottom int32 }
		ok, _, _ := getWindowRect.Call(uintptr(a.mini.NativeWindow()), uintptr(unsafe.Pointer(&r)))
		if ok != 0 {
			a.quickX, a.quickY = r.Left-12, r.Top-12
			a.quickCursor.Store(true)
		}
	})
	a.refreshQuickSuggestions()
}
func (a *App) quickLostFocus() {
	if a.quitting.Load() {
		return
	}
	generation := a.quickGeneration.Load()
	time.AfterFunc(80*time.Millisecond, func() {
		application.InvokeSync(func() {
			if a.quitting.Load() || generation != a.quickGeneration.Load() {
				return
			}
			foreground, _, _ := getForegroundWindow.Call()
			root, _, _ := user32.NewProc("GetAncestor").Call(foreground, 3)
			if foreground == uintptr(a.mini.NativeWindow()) || root == uintptr(a.mini.NativeWindow()) {
				return
			}
			a.quickGeneration.Add(1)
			a.hideQuickSuggestions()
			a.mini.Hide()
		})
	})
}

func quickSuggestionStyle() int       { return 0x08000000 | 0x00000008 | 0x00000080 | 0x00010000 }
func quickSuggestionsSupported() bool { return true }
func (a *App) captureQuickPoint() {
	application.InvokeSync(func() {
		var p struct{ X, Y int32 }
		if ok, _, _ := user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&p))); ok != 0 {
			a.quickX, a.quickY = p.X, p.Y
		}
	})
}
func (a *App) clampQuickToWorkArea() {
	application.InvokeSync(func() {
		packed := uintptr(uint64(uint32(a.quickX)) | uint64(uint32(a.quickY))<<32)
		monitor, _, _ := user32.NewProc("MonitorFromPoint").Call(packed, 2)
		var info struct {
			Size          uint32
			Monitor, Work struct{ Left, Top, Right, Bottom int32 }
			Flags         uint32
		}
		info.Size = uint32(unsafe.Sizeof(info))
		if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
			return
		}
		var r struct{ Left, Top, Right, Bottom int32 }
		if ok, _, _ := getWindowRect.Call(uintptr(a.mini.NativeWindow()), uintptr(unsafe.Pointer(&r))); ok == 0 {
			return
		}
		width, height := r.Right-r.Left, r.Bottom-r.Top
		x, y := r.Left, r.Top
		const margin = 8
		if x+width > info.Work.Right-margin {
			x = info.Work.Right - width - margin
		}
		if y+height > info.Work.Bottom-margin {
			y = info.Work.Bottom - height - margin
		}
		if x < info.Work.Left+margin {
			x = info.Work.Left + margin
		}
		if y < info.Work.Top+margin {
			y = info.Work.Top + margin
		}
		a.mini.SetBounds(application.PhysicalToDipRect(application.Rect{X: int(x), Y: int(y), Width: int(width), Height: int(height)}))
	})
}
func (a *App) positionQuickSuggestions() {
	application.InvokeSync(func() {
		v := a.GetQuickSuggestions()
		if len(v.Items) == 0 {
			a.hideQuickSuggestionWindow()
			return
		}
		if a.quitting.Load() || v.Generation != a.quickGeneration.Load() {
			return
		}
		mini := uintptr(a.mini.NativeWindow())
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(mini)
		if visible == 0 {
			a.hideQuickSuggestionWindow()
			return
		}
		monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(mini, 2)
		var info struct {
			Size          uint32
			Monitor, Work struct{ Left, Top, Right, Bottom int32 }
			Flags         uint32
		}
		info.Size = uint32(unsafe.Sizeof(info))
		if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
			return
		}
		dpi, _, _ := user32.NewProc("GetDpiForWindow").Call(mini)
		scale := float64(dpi) / 96
		if scale <= 0 {
			scale = 1
		}
		point := struct{ X, Y int32 }{int32(float64(v.Anchor.X) * scale), int32(float64(v.Anchor.Y) * scale)}
		user32.NewProc("ClientToScreen").Call(mini, uintptr(unsafe.Pointer(&point)))
		width := int(float64(v.Anchor.Width) * scale)
		height := int(float64(len(v.Items)*34+8) * scale)
		gap := int(4 * scale)
		bottom := int(point.Y) + int(float64(v.Anchor.Height)*scale) + gap
		above := int(point.Y) - gap - int(info.Work.Top)
		below := int(info.Work.Bottom) - bottom
		x, y := int(point.X), bottom
		if below < height && above > below {
			height = min(height, above)
			y = int(point.Y) - gap - height
		} else {
			height = min(height, below)
		}
		if height < 20 {
			a.hideQuickSuggestionWindow()
			return
		}
		x = max(int(info.Work.Left), min(x, int(info.Work.Right)-width))
		popup := uintptr(a.quickSuggestions.NativeWindow())
		user32.NewProc("SetWindowLongPtrW").Call(popup, ^uintptr(7), mini) // GWL_HWNDPARENT: owned popup.
		a.quickSuggestions.SetBounds(application.PhysicalToDipRect(application.Rect{X: x, Y: y, Width: width, Height: height}))
		registerPassiveWindow(a.quickSuggestions)
		a.quickSuggestions.Show()
		showWindow.Call(popup, 4)
		setWindowPos.Call(popup, ^uintptr(0), 0, 0, 0, 0, 0x1|0x2|0x10|0x40)
	})
}
