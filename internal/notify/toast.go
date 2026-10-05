// Package notify は Windows のアクションセンター通知（トースト）を出す薄いラッパー。
// git.sr.ht/~jackmordaunt/go-toast は Windows 以外では no-op ビルドになるため、
// このパッケージも OS 分岐なしでそのまま呼び出せる。
//
// 独立したWails v3通知ウィンドウが主通知で、標準通知は設定で有効にする補助手段。
package notify

import (
	"log"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
)

const appID = "Kotorima"

// Init はアプリ情報をWindowsに登録し、通知クリック時のコールバックを設定する。
// アプリ起動時に一度だけ呼び出す。
func Init(iconPath string, onActivated func()) {
	if err := toast.SetAppData(toast.AppData{
		AppID:    appID,
		IconPath: iconPath,
	}); err != nil {
		log.Printf("notify: SetAppData失敗: %v", err)
	}
	if onActivated != nil {
		toast.SetActivationCallback(func(args string, data []toast.UserData) {
			onActivated()
		})
	}
}

// Push はWindows通知（トースト）を表示する。失敗してもアプリの動作は継続する
// （独自通知ウィンドウが主表示）。
func Push(title, body string) {
	n := toast.Notification{
		AppID: appID,
		Title: title,
		Body:  body,
		Audio: toast.Default,
	}
	if err := n.Push(); err != nil {
		log.Printf("notify: 通知の表示に失敗しました: %v", err)
	}
}
