import { useEffect, useState } from "react";
import { api } from "./api";
import { aiKeySlot, JevEndpoint, jevProvider, JevStatus } from "./smartAdd";
const results: Record<string, string> = {
  ok: "接続できました",
  invalid: "キーが無効です",
  unreachable: "接続できません",
};
// Keys are saved per provider immediately and never come back from Go; only their last
// four characters do. The connection test uses the endpoint as entered, even unsaved.
export function JevKeyField({ endpoint }: { endpoint: JevEndpoint }) {
  return <JevKeyEditor key={aiKeySlot(endpoint)} endpoint={endpoint} />;
}
function JevKeyEditor({ endpoint }: { endpoint: JevEndpoint }) {
  const provider = jevProvider(endpoint.provider);
  const slot = aiKeySlot(endpoint);
  const local = provider.id === "local";
  const [status, setStatus] = useState<JevStatus | null>(null),
    [key, setKey] = useState(""),
    [message, setMessage] = useState(""),
    [busy, setBusy] = useState(false);
  const hasKey = status?.key_configured ?? status?.configured;
  const refresh = () =>
    api<JevStatus>("GetJevStatus", slot).then((s) => setStatus(s ?? null));
  useEffect(() => {
    setKey("");
    setMessage("");
    setStatus(null);
    void refresh().catch((e) => setMessage(String(e)));
  }, [slot]);
  const run = async (f: () => Promise<string>) => {
    setBusy(true);
    setMessage("");
    try {
      setMessage(await f());
      await refresh();
    } catch (e) {
      setMessage(String(e));
    } finally {
      setBusy(false);
    }
  };
  if (status && !status.supported && !local)
    return <p className="wide jev-note">この環境では使えません</p>;
  return (
    <div className="wide jev-key">
      <label>
        {provider.key}
        <input
          type="password"
          autoComplete="off"
          value={key}
          placeholder={
            hasKey ? `設定済み（末尾 ••••${status?.hint ?? ""}）` : "未設定"
          }
          onChange={(e) => setKey(e.target.value)}
        />
      </label>
      <div className="actions">
        <button
          disabled={busy || !key.trim() || !status?.supported}
          onClick={() =>
            void run(async () => {
              await api("SetJevKey", slot, key);
              setKey("");
              return "保存しました";
            })
          }
        >
          保存
        </button>
        <button
          disabled={busy || !hasKey}
          onClick={() =>
            void run(async () => {
              await api("ClearJevKey", slot);
              return "削除しました";
            })
          }
        >
          削除
        </button>
        <button
          disabled={busy || (!local && !hasKey)}
          onClick={() =>
            void run(
              async () =>
                results[
                  await api<string>("TestJevKey", {
                    ...endpoint,
                    provider: provider.id,
                  })
                ] ?? "",
            )
          }
        >
          接続確認
        </button>
      </div>
      {status?.invalid && (
        <p className="danger">キーが無効です。保存し直してください。</p>
      )}
      {status?.unavailable && !status.invalid && (
        <p role="status">
          AIとの接続を停止しています。接続確認に成功すると再開します。
        </p>
      )}
      {message && <p role="status">{message}</p>}
      <p className="jev-note">
        入力中のタイトル・未完了タスク名・カテゴリ名を {provider.sendTo}{" "}
        に送信します。
        {local &&
          "別途AIサーバーを起動し、モデルを読み込んでください。認証なしならキーは不要です。"}
        APIキーはこのPCのユーザーで暗号化して保存し、バックアップには含めません（別のPCへ復元したときは再入力が必要です）。
      </p>
    </div>
  );
}
