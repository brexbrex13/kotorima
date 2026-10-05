import { useEffect, useState } from "react";
import { api } from "./api";

export function ImageViewer() {
  const src = new URLSearchParams(location.search).get("src") || "";
  const [error, setError] = useState("");
  const close = () =>
    void api("CloseImageViewer").catch((e) => setError(String(e)));
  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  }, []);
  return (
    <main className="image-viewer">
      <header style={{ "--wails-draggable": "drag" } as React.CSSProperties}>
        <button aria-label="画像ビューアを閉じる" autoFocus onClick={close}>
          ×
        </button>
      </header>
      {error ? (
        <p role="alert">{error}</p>
      ) : (
        <img
          src={src}
          alt="メモの添付画像"
          onError={() => setError("画像を読み込めませんでした")}
        />
      )}
    </main>
  );
}
