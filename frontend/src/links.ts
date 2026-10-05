// File links store references, not attachment copies. Preserve spaces and UNC.
export function memoLink(value: string): string {
  const trimmed = value.trim();
  const v =
    trimmed.startsWith('"') && trimmed.endsWith('"')
      ? trimmed.slice(1, -1).trim()
      : trimmed;
  if (/^https?:\/\//i.test(v) || /^file:\/\//i.test(v)) return v;
  if (/^[a-z]:[\\/]/i.test(v))
    return (
      "file:///" +
      v
        .replace(/\\/g, "/")
        .split("/")
        .map((part, i) => (i === 0 ? part : encodeURIComponent(part)))
        .join("/")
    );
  if (v.startsWith("\\\\")) {
    const [host, ...parts] = v.slice(2).split("\\");
    return "file://" + host + "/" + parts.map(encodeURIComponent).join("/");
  }
  throw new Error("HTTP/HTTPS、ローカルファイル、UNCパスを入力してください");
}
