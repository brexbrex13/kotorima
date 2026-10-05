import { useMemo } from "react";
import { api } from "./api";
import { sanitizeMemo } from "./memo";

export function MemoPreview({
  value,
  onError,
}: {
  value: string;
  onError: (error: unknown) => void;
}) {
  const html = useMemo(() => {
    const template = document.createElement("template");
    template.innerHTML = sanitizeMemo(value);
    template.content
      .querySelectorAll("form,button,select,textarea")
      .forEach((el) => el.remove());
    template.content.querySelectorAll("input").forEach((el) => {
      el.disabled = true;
    });
    template.content
      .querySelectorAll("[contenteditable]")
      .forEach((el) => el.removeAttribute("contenteditable"));
    template.content.querySelectorAll("img").forEach((el) => {
      el.tabIndex = 0;
      el.setAttribute("role", "button");
      el.setAttribute("aria-label", "添付画像を拡大");
    });
    return template.innerHTML;
  }, [value]);
  return (
    <>
      <div
        className="notice-memo"
        aria-label="通知のメモ"
        onClick={(e) => {
          const el = e.target as HTMLElement;
          const link = el.closest("a");
          if (link) {
            e.preventDefault();
            void api("OpenURL", link.href).catch(onError);
          } else if (el.tagName === "IMG") {
            void api("OpenImageViewer", el.getAttribute("src") ?? "").catch(
              onError,
            );
          }
        }}
        onKeyDown={(e) => {
          const el = e.target as HTMLElement;
          if (el.tagName === "IMG" && (e.key === "Enter" || e.key === " ")) {
            e.preventDefault();
            el.click();
          }
        }}
        dangerouslySetInnerHTML={{ __html: html }}
      />
    </>
  );
}
