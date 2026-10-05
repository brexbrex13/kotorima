import { useEffect, useRef, useState } from "react";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import { TextStyle } from "@tiptap/extension-text-style";
import Color from "@tiptap/extension-color";
import Image from "@tiptap/extension-image";
import Link from "@tiptap/extension-link";
import TaskList from "@tiptap/extension-task-list";
import TaskItem from "@tiptap/extension-task-item";
import { Icon } from "./Icons";
import { memoLink } from "./links";
import { api } from "./api";
import { sanitizeMemo } from "./memo";
export function Editor({
  value,
  onChange,
  onError,
  onBusy,
}: {
  value: string;
  onChange: (v: string) => void;
  onError: (e: unknown) => void;
  onBusy?: (v: boolean) => void;
}) {
  const change = useRef(onChange);
  change.current = onChange;
  const file = useRef<HTMLInputElement>(null);
  const uploads = useRef(0);
  const resize = useRef<HTMLDivElement>(null);
  const [height] = useState(() =>
    Math.max(
      64,
      Math.min(900, Number(localStorage.getItem("memo-height-small")) || 80),
    ),
  );

  const upload = async (blob: Blob) => {
    uploads.current++;
    onBusy?.(true);
    try {
      const data = await new Promise<string>((resolve, reject) => {
        const r = new FileReader();
        r.onload = () => resolve(String(r.result));
        r.onerror = reject;
        r.readAsDataURL(blob);
      });
      const src = await api<string>("SaveImage", data);
      if (editor)
        editor
          .chain()
          .focus()
          .insertContentAt(editor.state.selection.to, {
            type: "image",
            attrs: { src },
          })
          .run();
    } catch (e) {
      onError(e);
    } finally {
      uploads.current--;
      onBusy?.(uploads.current > 0);
    }
  };
  const editor = useEditor({
    extensions: [
      StarterKit.configure({ link: false }),
      TextStyle,
      Color,
      Image,
      Link.configure({
        openOnClick: false,
        autolink: true,
        protocols: ["file"],
        isAllowedUri: (url) => /^(https?:\/\/|file:\/\/)/i.test(url),
      }),
      TaskList,
      TaskItem.configure({ nested: true }),
    ],
    // Keep existing list/heading nodes readable, without creation commands.
    enableInputRules: false,
    enablePasteRules: false,
    content: sanitizeMemo(value),
    onUpdate: ({ editor }) => change.current(editor.getHTML()),
    editorProps: {
      attributes: {
        role: "textbox",
        "aria-label": "メモ",
        "aria-multiline": "true",
      },
      handleKeyDown: (_view, e) => {
        if (
          (e.ctrlKey || e.metaKey) &&
          ((e.shiftKey && /^Digit[7-9]$/.test(e.code)) ||
            (e.altKey && /^Digit[1-6]$/.test(e.code)))
        ) {
          e.preventDefault();
          return true;
        }
        return false;
      },
      handlePaste: (_view, e) => {
        const item = Array.from(e.clipboardData?.items ?? []).find((i) =>
          i.type.startsWith("image/"),
        );
        if (item) {
          const blob = item.getAsFile();
          if (blob) {
            void upload(blob);
            return true;
          }
        }
        const text = e.clipboardData?.getData("text/plain")?.trim() || "";
        if (text && !/[\r\n]/.test(text)) {
          try {
            const href = memoLink(text);
            const label = text.replace(/^"(.*)"$/, "$1");
            const mark = _view.state.schema.marks.link.create({ href });
            _view.dispatch(
              _view.state.tr
                .replaceSelectionWith(
                  _view.state.schema.text(label, [mark]),
                  false,
                )
                .setStoredMarks(null),
            );
            return true;
          } catch {
            /* Ordinary text keeps its original paste behavior. */
          }
        }
        return false;
      },
      handleDrop: (_view, e) => {
        const f = e.dataTransfer?.files[0];
        if (f?.type.startsWith("image/")) {
          void upload(f);
          return true;
        }
        return false;
      },
      handleClick: (_view, _pos, e) => {
        const el = e.target as HTMLElement;
        if (el.tagName === "IMG") {
          void api("OpenImageViewer", el.getAttribute("src") ?? "").catch(
            onError,
          );
          return true;
        }
        const a = el.closest("a");
        if (a) {
          void api("OpenURL", a.href).catch(onError);
          return true;
        }
        return false;
      },
    },
  });
  useEffect(() => {
    if (editor && value !== editor.getHTML() && !editor.isFocused)
      editor.commands.setContent(sanitizeMemo(value), {
        emitUpdate: false,
      });
  }, [value, editor]);
  useEffect(() => {
    const el = resize.current;
    if (!el) return;
    const persist = () =>
      localStorage.setItem(
        "memo-height-small",
        String(Math.round(el.getBoundingClientRect().height)),
      );
    el.addEventListener("pointerup", persist);
    const obs = new ResizeObserver(persist);
    obs.observe(el);
    return () => {
      el.removeEventListener("pointerup", persist);
      obs.disconnect();
    };
  }, [editor]);
  if (!editor) return null;
  return (
    <div className="memo">
      <div className="toolbar">
        <button
          type="button"
          aria-label="太字"
          data-tip="太字 Ctrl+B"
          aria-pressed={editor.isActive("bold")}
          onClick={() => editor.chain().focus().toggleBold().run()}
        >
          <b>B</b>
        </button>
        <button
          type="button"
          aria-label="斜体"
          data-tip="斜体 Ctrl+I"
          aria-pressed={editor.isActive("italic")}
          onClick={() => editor.chain().focus().toggleItalic().run()}
        >
          <i>I</i>
        </button>
        <div className="text-colors" role="group" aria-label="文字色">
          {[
            ["標準", "#302d25"],
            ["赤", "#b32929"],
            ["青", "#185caa"],
            ["緑", "#26713e"],
            ["紫", "#743c91"],
          ].map(([name, color]) => (
            <button
              key={name}
              type="button"
              aria-label={"文字色：" + name}
              data-tip={"文字色：" + name}
              style={{ color }}
              onClick={() => editor.chain().focus().setColor(color).run()}
            >
              A
            </button>
          ))}
        </div>
        <button
          type="button"
          aria-label="リンク"
          data-tip="リンクを追加・変更"
          onClick={() => {
            const href = prompt(
              "リンク先（HTTP/HTTPS、ローカル・UNCパス）",
              editor.getAttributes("link").href ?? "",
            );
            if (href === null) return;
            if (!href) {
              editor.chain().focus().unsetLink().run();
              return;
            }
            let target: string;
            try {
              target = memoLink(href);
            } catch (e) {
              onError(e);
              return;
            }
            if (editor.state.selection.empty && !editor.isActive("link")) {
              editor
                .chain()
                .focus()
                .insertContent({
                  type: "text",
                  text: href.trim().replace(/^"(.*)"$/, "$1"),
                  marks: [{ type: "link", attrs: { href: target } }],
                })
                .command(({ tr }) => {
                  tr.setStoredMarks([]);
                  return true;
                })
                .run();
            } else
              editor
                .chain()
                .focus()
                .extendMarkRange("link")
                .setLink({ href: target })
                .run();
          }}
        >
          <Icon name="link" />
        </button>
        <button
          type="button"
          aria-label="画像添付"
          data-tip="画像を本文に挿入します。Ctrl+V・ドロップでも添付できます"
          onClick={() => file.current?.click()}
        >
          <Icon name="image" />
        </button>
        <button
          type="button"
          aria-label="元に戻す"
          data-tip="元に戻す Ctrl+Z"
          onClick={() => editor.chain().focus().undo().run()}
        >
          <Icon name="undo" />
        </button>
        <button
          type="button"
          aria-label="やり直す"
          data-tip="やり直す Ctrl+Shift+Z"
          onClick={() => editor.chain().focus().redo().run()}
        >
          <Icon name="redo" />
        </button>
        <input
          ref={file}
          hidden
          type="file"
          accept="image/png,image/jpeg,image/gif,image/webp"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) void upload(f);
            e.target.value = "";
          }}
        />
      </div>
      <div className="memo-editor" ref={resize} style={{ height }}>
        <EditorContent editor={editor} />
      </div>
    </div>
  );
}
