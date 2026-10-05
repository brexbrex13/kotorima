import { useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export function ContextMenu({
  x,
  y,
  label,
  items,
  onClose,
}: {
  x: number;
  y: number;
  label: string;
  items: { label: string; action: () => void }[];
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  const [position, setPosition] = useState({ left: x, top: y });
  useLayoutEffect(() => {
    const menu = ref.current!;
    const previous = document.activeElement as HTMLElement | null;
    const rect = menu.getBoundingClientRect();
    setPosition({
      left: Math.max(8, Math.min(x, window.innerWidth - rect.width - 8)),
      top: Math.max(8, Math.min(y, window.innerHeight - rect.height - 8)),
    });
    menu
      .querySelector<HTMLButtonElement>("button")
      ?.focus({ preventScroll: true });
    const dismiss = () => close.current();
    const outside = (e: PointerEvent) => {
      if (!menu.contains(e.target as Node)) dismiss();
    };
    const keyboard = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
        dismiss();
        previous?.focus({ preventScroll: true });
      } else if (e.key === "Tab") {
        dismiss();
      } else if (["ArrowDown", "ArrowUp", "Home", "End"].includes(e.key)) {
        e.preventDefault();
        const buttons = Array.from(menu.querySelectorAll("button"));
        const index = buttons.indexOf(
          document.activeElement as HTMLButtonElement,
        );
        const next =
          e.key === "Home"
            ? 0
            : e.key === "End"
              ? buttons.length - 1
              : (index + (e.key === "ArrowDown" ? 1 : -1) + buttons.length) %
                buttons.length;
        buttons[next]?.focus({ preventScroll: true });
      }
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", keyboard, true);
    window.addEventListener("resize", dismiss);
    // Ignore scroll events queued while the right-click target was brought into view.
    const scrollFrame = requestAnimationFrame(() => {
      document.addEventListener("scroll", dismiss, true);
    });
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", keyboard, true);
      window.removeEventListener("resize", dismiss);
      cancelAnimationFrame(scrollFrame);
      document.removeEventListener("scroll", dismiss, true);
    };
  }, [x, y]);
  return createPortal(
    <div
      ref={ref}
      className="category-context"
      role="menu"
      aria-label={label}
      style={{ ...position, width: "max-content", maxWidth: "calc(100vw - 16px)" }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item) => (
        <button
          key={item.label}
          role="menuitem"
          onClick={() => {
            onClose();
            item.action();
          }}
        >
          {item.label}
        </button>
      ))}
    </div>,
    document.body,
  );
}
