# ビルド資産

`appicon.png`はアプリ・補助通知用、`windows/icon.ico`はWindows実行ファイル用です。ICOには16・24・32・48・64・128・256pxを含めます。

`windows/winres.json`からgo-winresでアイコン・バージョン・DPI対応manifestをsysoへ埋め込みます。ビルド・配布は`docs/release.md`とGitHub Actionsを参照してください。
