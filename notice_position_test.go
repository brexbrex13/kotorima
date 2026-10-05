package main

import (
	"kotorima/internal/board"
	"testing"
)

func TestNoticeSizeRespectsMemoAndPrivacy(t *testing.T) {
	task := board.Task{ID: 1, Memo: "<p>作業手順</p>", ShowMemoInNotice: true}
	notice := board.Notification{TaskID: 1}
	v := board.Snapshot{Tasks: []board.Task{task}}
	w, h := noticeSize(notice, v)
	if w != 360 || h != 330 {
		t.Fatal(w, h)
	}
	v.Settings.Private = true
	_, h = noticeSize(notice, v)
	if h != 170 {
		t.Fatal("private notice grew for a hidden memo", h)
	}
	v.Settings.Private = false
	v.Tasks[0].ShowMemoInNotice = false
	_, h = noticeSize(notice, v)
	if h != 170 {
		t.Fatal(h)
	}
	v.Tasks[0].ShowMemoInNotice = true
	v.Tasks[0].Memo = ""
	_, h = noticeSize(notice, v)
	if h != 170 {
		t.Fatal(h)
	}
	w, h = noticeSize(board.Notification{Kind: "summary"}, v)
	if w != 440 || h != 390 {
		t.Fatal(w, h)
	}
}
