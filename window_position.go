package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"kotorima/internal/board"
)

// Choose the work area with the greatest overlap. A disconnected monitor falls
// back to the primary screen; negative coordinates remain valid on other screens.
func fitMainBounds(v board.WindowBounds, areas []application.Rect) board.WindowBounds {
	if len(areas) == 0 {
		return v
	}
	chosen, overlap := areas[0], 0
	for _, r := range areas {
		w := max(0, min(v.X+v.Width, r.X+r.Width)-max(v.X, r.X))
		h := max(0, min(v.Y+v.Height, r.Y+r.Height)-max(v.Y, r.Y))
		if w*h > overlap {
			chosen, overlap = r, w*h
		}
	}
	v.Width = min(v.Width, chosen.Width)
	v.Height = min(v.Height, chosen.Height)
	if overlap == 0 {
		v.X = chosen.X + (chosen.Width-v.Width)/2
		v.Y = chosen.Y + (chosen.Height-v.Height)/2
	} else {
		v.X = max(chosen.X, min(v.X, chosen.X+chosen.Width-v.Width))
		v.Y = max(chosen.Y, min(v.Y, chosen.Y+chosen.Height-v.Height))
	}
	return v
}

func (a *App) saveMainWindowBounds() error {
	if a.main == nil {
		return nil
	}
	x, y := a.main.Position()
	w, h := a.main.Size()
	return a.store.SaveWindowBounds(board.WindowBounds{X: x, Y: y, Width: w, Height: h})
}

func (a *App) restoreMainWindowBounds() {
	v, ok := a.store.WindowBounds()
	if !ok {
		return
	}
	v = fitMainBounds(v, a.mainWorkAreas())
	a.main.SetBounds(application.Rect{X: v.X, Y: v.Y, Width: v.Width, Height: v.Height})
}

func (a *App) mainWorkAreas() []application.Rect {
	areas := []application.Rect{}
	if s := a.desktop.Screen.GetPrimary(); s != nil {
		areas = append(areas, s.WorkArea)
	}
	for _, s := range a.desktop.Screen.GetAll() {
		areas = append(areas, s.WorkArea)
	}
	return areas
}

// Do not move a user-positioned window while its title bar is reachable. When a
// monitor disappears while the app is in the tray, bring it back onto a screen.
func (a *App) ensureMainVisible() {
	x, y := a.main.Position()
	w, h := a.main.Size()
	areas := a.mainWorkAreas()
	for _, r := range areas {
		if min(x+w, r.X+r.Width)-max(x, r.X) >= min(w, 100) && y+32 > r.Y && y < r.Y+r.Height {
			return
		}
	}
	v := fitMainBounds(board.WindowBounds{X: x, Y: y, Width: w, Height: h}, areas)
	a.main.SetBounds(application.Rect{X: v.X, Y: v.Y, Width: v.Width, Height: v.Height})
}
