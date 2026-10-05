import { useEffect, useState } from "react";
import { createPortal } from "react-dom";

export function Tooltip() {
  const [tip, setTip] = useState<{ text: string; x: number; y: number } | null>(
    null,
  );
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    let active: Element | null = null;
    const clear = () => {
      clearTimeout(timer);
      active = null;
      setTip(null);
    };
    const enter = (e: Event) => {
      const target = (e.target as Element)?.closest?.("[data-tip], [title]");
      if (target === active) return;
      clear();
      if (
        !target ||
        (target.closest(".quick, .mini-input") &&
          document.querySelector(".task-suggestions"))
      )
        return;
      const text =
        target.getAttribute("data-tip") || target.getAttribute("title");
      if (!text) return;
      target.setAttribute("data-tip", text);
      target.removeAttribute("title");
      active = target;
      timer = setTimeout(() => {
        if (
          target.closest(".quick, .mini-input") &&
          document.querySelector(".task-suggestions")
        )
          return;
        const r = target.getBoundingClientRect();
        setTip({
          text,
          x: Math.max(8, Math.min(r.left, window.innerWidth - 288)),
          y: r.bottom + 7,
        });
      }, 200);
    };
    const leave = (e: MouseEvent) => {
      if (
        active &&
        !(e.relatedTarget instanceof Node && active.contains(e.relatedTarget))
      )
        clear();
    };
    document.addEventListener("mouseover", enter);
    document.addEventListener("focusin", enter);
    document.addEventListener("mouseout", leave);
    document.addEventListener("focusout", clear);
    document.addEventListener("pointerdown", clear);
    document.addEventListener("keydown", clear);
    window.addEventListener("scroll", clear, true);
    return () => {
      clearTimeout(timer);
      document.removeEventListener("mouseover", enter);
      document.removeEventListener("focusin", enter);
      document.removeEventListener("mouseout", leave);
      document.removeEventListener("focusout", clear);
      document.removeEventListener("pointerdown", clear);
      document.removeEventListener("keydown", clear);
      window.removeEventListener("scroll", clear, true);
    };
  }, []);
  return tip
    ? createPortal(
        <div
          role="tooltip"
          className="tip"
          style={{
            left: tip.x,
            top: tip.y > window.innerHeight - 95 ? undefined : tip.y,
            bottom: tip.y > window.innerHeight - 95 ? 8 : undefined,
          }}
        >
          {tip.text}
        </div>,
        document.body,
      )
    : null;
}
