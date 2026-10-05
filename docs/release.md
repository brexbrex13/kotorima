# Windowsビルドの配布

`.github/workflows/ci.yml`でロジック・画面・Windowsネイティブ動作を検証し、ポータブルZIPを作成します。

`build/version.txt`を更新してmainへ取り込むと、Releaseワークフローが実行されます。バージョンは`build/windows/winres.json`にも反映します。検証成功後に`vX.Y.Z`タグと公開Releaseを作成します。同じバージョンは上書きしません。手動実行・`v*`タグにも対応します。

配布物は`kotorima-windows-amd64.zip`です。Windows 10/11とWebView2 Runtimeが必要です。依存関係とWailsコミットは固定しています。保存先は実行ファイル横のdataフォルダーで、MemoTodo v3のデータ・バックアップ形式を維持しています。
