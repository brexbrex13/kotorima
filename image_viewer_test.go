package main

import (
	"net/url"
	"testing"
)

func TestImageViewerURL(t *testing.T) {
	for _, src := range []string{"/images/example.png", "https://example.com/a.png?name=a&x=1"} {
		route, err := imageViewerURL(src)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(route)
		if u.Query().Get("src") != src || u.Query().Get("window") != "image-viewer" {
			t.Fatal(route)
		}
	}
	for _, src := range []string{"javascript:alert(1)", "file:///C:/secret.png", "/images/..", "/images/../secret.png", "/images/a%2Fsecret.png", "//example.com/x", ""} {
		if _, err := imageViewerURL(src); err == nil {
			t.Fatal("accepted", src)
		}
	}
}
