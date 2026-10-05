//go:build !windows || server

package main

type shortcutManager struct{}

func newShortcutManager(func()) *shortcutManager { return &shortcutManager{} }
func (*shortcutManager) Change(string) error     { return nil }
func (*shortcutManager) Close()                  {}
func (*App) rememberQuickFocus()                 {}
func (a *App) hideQuickNative(generation uint64) {
	if generation == a.quickGeneration.Load() {
		a.hideQuickSuggestions()
		a.mini.Hide()
	}
}

func (*App) placeQuickAtCursor(bool) {}
func (*App) quickLostFocus()         {}

func (*App) rememberQuickPosition() {}

func quickSuggestionStyle() int        { return 0 }
func quickSuggestionsSupported() bool  { return false }
func (*App) positionQuickSuggestions() {}
func (*App) captureQuickPoint()        {}
func (*App) clampQuickToWorkArea()     {}
