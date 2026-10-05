package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"kotorima/internal/board"
	"testing"
)

func TestMainBoundsRestoreAndClamp(t *testing.T) {
	areas := []application.Rect{{X: 0, Y: 0, Width: 1920, Height: 1040}, {X: -1920, Y: 0, Width: 1920, Height: 1040}}
	for _, tc := range []struct{ saved, want board.WindowBounds }{
		{board.WindowBounds{X: -1700, Y: 100, Width: 960, Height: 700}, board.WindowBounds{X: -1700, Y: 100, Width: 960, Height: 700}},
		{board.WindowBounds{X: 5000, Y: 100, Width: 960, Height: 700}, board.WindowBounds{X: 480, Y: 170, Width: 960, Height: 700}},
		{board.WindowBounds{X: 1700, Y: 900, Width: 960, Height: 700}, board.WindowBounds{X: 960, Y: 340, Width: 960, Height: 700}},
		{board.WindowBounds{X: 0, Y: 0, Width: 3000, Height: 2000}, board.WindowBounds{X: 0, Y: 0, Width: 1920, Height: 1040}},
	} {
		if got := fitMainBounds(tc.saved, areas); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
	dir := t.TempDir()
	s, e := board.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	v := board.WindowBounds{X: -1500, Y: 80, Width: 720, Height: 600}
	if e = s.SaveWindowBounds(v); e != nil {
		t.Fatal(e)
	}
	s.Close()
	s, e = board.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if got, ok := s.WindowBounds(); !ok || got != v {
		t.Fatal(got, ok)
	}
}
