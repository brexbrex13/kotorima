package main

import (
	"encoding/base64"
	"kotorima/internal/board"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImagesUseValidatedTypeAndIndependentFilenames(t *testing.T) {
	s, e := board.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	a := &App{store: s}
	encoded := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n"+strings.Repeat("\x00", 32)))
	first, e := a.SaveImage(encoded)
	if e != nil {
		t.Fatal(e)
	}
	second, e := a.SaveImage(encoded)
	if e != nil {
		t.Fatal(e)
	}
	if first == second || !strings.HasPrefix(first, "/images/") || !strings.HasSuffix(first, ".png") {
		t.Fatal(first, second)
	}
	if _, e = os.Stat(filepath.Join(s.Dir, strings.TrimPrefix(first, "/"))); e != nil {
		t.Fatal(e)
	}
	if _, e = a.SaveImage("data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("<script>bad</script>"))); e == nil {
		t.Fatal("non-image content accepted")
	}
}
