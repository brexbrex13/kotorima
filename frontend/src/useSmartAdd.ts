import { useEffect, useRef, useState } from "react";
import { Task } from "./types";
import {
  applySuggestion,
  AutoFields,
  chipLabels,
  dropChip,
  Field,
  QuickOptions,
  Suggestion,
} from "./smartAdd";
// Shared by the quick-add window and the main board input.
// Automatic values never overwrite fields the user touched; an answer is dropped when the
// title, the request sequence or the caller's session changed before it arrived.
export function useSmartAdd({
  text,
  options,
  setOptions,
  enabled,
  defaultTime,
  categories,
  ask,
  session = () => "",
  scope = "",
  hide = [],
  existingTasks = [],
}: {
  text: string;
  options: QuickOptions;
  setOptions: (options: QuickOptions) => void;
  enabled: boolean;
  defaultTime: string;
  categories: { id: number; name: string }[];
  ask: (title: string) => Promise<Suggestion>;
  session?: () => string;
  scope?: string;
  hide?: Field[];
  existingTasks?: Task[];
}) {
  const [auto, setAuto] = useState<AutoFields>({}),
    [duplicate, setDuplicate] = useState<Suggestion["duplicate"]>(null),
    [invalid, setInvalid] = useState(false),
    [unavailable, setUnavailable] = useState(false),
    [composeTick, setComposeTick] = useState(0);
  const touched = useRef(new Set<Field>()),
    autoRef = useRef<AutoFields>({}),
    optionsRef = useRef(options),
    textRef = useRef(text),
    composing = useRef(false),
    lastAsked = useRef(""),
    seq = useRef(0);
  const autoTitle = useRef(text.trim());
  optionsRef.current = options;
  textRef.current = text;
  const commit = (next: { options: QuickOptions; auto: AutoFields }) => {
    autoTitle.current = textRef.current.trim();
    autoRef.current = next.auto;
    optionsRef.current = next.options;
    setAuto(next.auto);
    setOptions(next.options);
  };
  const reset = () => {
    seq.current++;
    touched.current = new Set();
    lastAsked.current = "";
    autoRef.current = {};
    setAuto({});
    setDuplicate(null);
    setInvalid(false);
    setUnavailable(false);
  };
  const touch = (field: Field) => {
    touched.current.add(field);
    const next = { ...autoRef.current };
    // Editing a linked notification also accepts the date it depends on.
    if (
      field === "reminder" &&
      optionsRef.current.reminder_mode === "deadline" &&
      autoRef.current.deadline !== undefined
    ) {
      touched.current.add("deadline");
      delete next.deadline;
    }
    delete next[field];
    autoRef.current = next;
    setAuto(next);
  };
  const drop = (field: Field) => {
    touched.current.add(field);
    commit(dropChip(optionsRef.current, autoRef.current, field));
  };
  useEffect(() => {
    seq.current++;
    lastAsked.current = "";
    setInvalid(false);
    setUnavailable(false);
    setDuplicate(null);
    if (Object.keys(autoRef.current).length) {
      commit(
        applySuggestion(
          optionsRef.current,
          touched.current,
          autoRef.current,
          null,
          defaultTime,
        ),
      );
    }
  }, [scope, enabled]);
  useEffect(() => {
    if (autoTitle.current !== text.trim()) {
      commit(
        applySuggestion(
          optionsRef.current,
          touched.current,
          autoRef.current,
          null,
          defaultTime,
        ),
      );
      setDuplicate(null);
    }
  }, [text]);
  useEffect(() => {
    const title = text.trim();
    const key = scope + "\u0000" + title;
    if (!title) {
      lastAsked.current = "";
      if (Object.keys(autoRef.current).length)
        commit(
          applySuggestion(
            optionsRef.current,
            touched.current,
            autoRef.current,
            null,
            defaultTime,
          ),
        );
      setDuplicate(null);
      return;
    }
    if (
      !enabled ||
      invalid ||
      unavailable ||
      title.length < 2 ||
      key === lastAsked.current
    )
      return;
    const timer = setTimeout(() => {
      if (composing.current) return;
      lastAsked.current = key;
      const n = ++seq.current,
        started = session();
      void ask(title)
        .then((s) => {
          if (
            !s ||
            title !== textRef.current.trim() ||
            n !== seq.current ||
            started !== session()
          )
            return;
          setInvalid(!!s.invalid);
          setUnavailable(!!s.unavailable);
          commit(
            applySuggestion(
              optionsRef.current,
              touched.current,
              autoRef.current,
              s,
              defaultTime,
            ),
          );
          setDuplicate(s.duplicate);
        })
        .catch(() => {
          if (n === seq.current) lastAsked.current = "";
        });
    }, 400);
    return () => {
      clearTimeout(timer);
      seq.current++;
    };
  }, [text, enabled, invalid, unavailable, composeTick, scope]);
  const exact =
    enabled && text.trim()
      ? existingTasks.find(
          (t) =>
            t.status === "pending" &&
            !t.deleted_at &&
            t.title.trim().toLocaleLowerCase() ===
              text.trim().toLocaleLowerCase(),
        )
      : undefined;
  return {
    chips: chipLabels(auto, options, categories).filter(
      (c) => !hide.includes(c.field),
    ),
    duplicate: exact
      ? { task_id: exact.id, title: exact.title }
      : autoTitle.current === text.trim()
        ? duplicate
        : null,
    invalid,
    unavailable,
    currentOptions: () =>
      autoTitle.current === textRef.current.trim()
        ? optionsRef.current
        : applySuggestion(
            optionsRef.current,
            touched.current,
            autoRef.current,
            null,
            defaultTime,
          ).options,
    isAutomatic: (field: Field) =>
      autoTitle.current === textRef.current.trim() &&
      autoRef.current[field] !== undefined,
    setInvalid,
    touch,
    drop,
    reset,
    compositionStart: () => {
      composing.current = true;
    },
    compositionEnd: () => {
      composing.current = false;
      setComposeTick((n) => n + 1);
    },
  };
}
export type SmartAdd = ReturnType<typeof useSmartAdd>;
