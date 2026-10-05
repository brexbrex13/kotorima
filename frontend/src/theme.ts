import { useEffect } from "react";
export type Theme = "light" | "dark" | "system";
export function applyTheme(theme: Theme) {
  const dark =
    theme === "dark" ||
    (theme === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
  localStorage.setItem("theme", theme);
}
export function useTheme(theme?: Theme) {
  useEffect(() => {
    if (theme === undefined) return;
    const mode = theme || "light";
    applyTheme(mode);
    const media = matchMedia("(prefers-color-scheme: dark)");
    const change = () => applyTheme(mode);
    media.addEventListener("change", change);
    return () => media.removeEventListener("change", change);
  }, [theme]);
}
