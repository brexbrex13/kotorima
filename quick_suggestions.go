package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

type QuickSuggestions struct {
	Items      []string         `json:"items"`
	Index      int              `json:"index"`
	Revision   int64            `json:"revision"`
	Generation uint64           `json:"generation"`
	Anchor     application.Rect `json:"anchor"`
}

func (a *App) GetQuickSuggestions() QuickSuggestions {
	a.quickSuggestionMu.Lock()
	defer a.quickSuggestionMu.Unlock()
	v := a.quickSuggestionData
	v.Items = append([]string{}, v.Items...)
	return v
}
func (a *App) SetQuickSuggestions(v QuickSuggestions) bool {
	supported := quickSuggestionsSupported()
	if v.Generation != a.quickGeneration.Load() || a.quitting.Load() {
		return supported
	}
	if len(v.Items) > 5 {
		v.Items = v.Items[:5]
	}
	a.quickSuggestionMu.Lock()
	if v.Generation != a.quickGeneration.Load() {
		a.quickSuggestionMu.Unlock()
		return supported
	}
	if a.quickSuggestionData.Generation == v.Generation && v.Revision < a.quickSuggestionData.Revision {
		a.quickSuggestionMu.Unlock()
		return supported
	}
	v.Items = append([]string{}, v.Items...)
	a.quickSuggestionData = v
	a.quickSuggestionMu.Unlock()
	a.desktop.Event.Emit("board:mini-suggestions")
	a.refreshQuickSuggestions()
	return supported
}
func (a *App) refreshQuickSuggestions() {
	if a.quickSuggestions == nil || !a.quickSuggestionReady.Load() || a.quitting.Load() {
		return
	}
	a.positionQuickSuggestions()
}
func (a *App) hideQuickSuggestionWindow() {
	if a.quickSuggestions != nil {
		a.quickSuggestions.Hide()
	}
}
func (a *App) hideQuickSuggestions() {
	a.quickSuggestionMu.Lock()
	a.quickSuggestionData.Items = []string{}
	a.quickSuggestionMu.Unlock()
	a.hideQuickSuggestionWindow()
}
func (a *App) ChooseQuickSuggestion(title string) {
	v := a.GetQuickSuggestions()
	if a.quitting.Load() || v.Generation != a.quickGeneration.Load() {
		return
	}
	for _, item := range v.Items {
		if item == title {
			a.hideQuickSuggestions()
			a.desktop.Event.Emit("board:mini-choice", map[string]any{"title": title, "generation": v.Generation})
			a.mini.Focus()
			return
		}
	}
}
