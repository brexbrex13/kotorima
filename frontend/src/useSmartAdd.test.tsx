import { act, renderHook, waitFor } from "@testing-library/react";
import { expect, it } from "vitest";
import { useState } from "react";
import { useSmartAdd } from "./useSmartAdd";
import { blankOptions, Suggestion } from "./smartAdd";
import { emptyTask } from "./types";
const suggestion: Suggestion = {
  category_id: 0,
  important: true,
  deadline: "",
  reminder: "",
  duplicate: null,
  invalid: false,
};
it("drops an in-flight response after disabling AI or changing endpoint", async () => {
  for (const change of [
    { enabled: false, scope: "a" },
    { enabled: true, scope: "b" },
  ]) {
    let resolve!: (s: Suggestion) => void;
    let requested = false;
    const { result, rerender, unmount } = renderHook(
      ({ enabled, scope }) => {
        const [options, setOptions] = useState(blankOptions);
        return {
          options,
          smart: useSmartAdd({
            text: "提出する",
            options,
            setOptions,
            enabled,
            scope,
            defaultTime: "09:00",
            categories: [],
            ask: () => {
              requested = true;
              return new Promise<Suggestion>((r) => {
                resolve = r;
              });
            },
          }),
        };
      },
      { initialProps: { enabled: true, scope: "a" } },
    );
    await waitFor(() => expect(requested).toBe(true));
    rerender(change);
    await act(async () => resolve(suggestion));
    expect(result.current.options.important).toBe(false);
    unmount();
  }
});
it("resumes after an invalid key is replaced and clears automatic fields when disabled", async () => {
  let calls = 0;
  const { result, rerender } = renderHook(
    ({ enabled, scope }) => {
      const [options, setOptions] = useState(blankOptions);
      return {
        options,
        smart: useSmartAdd({
          text: "提出する",
          options,
          setOptions,
          enabled,
          scope,
          defaultTime: "09:00",
          categories: [],
          ask: async () =>
            ++calls === 1 ? { ...suggestion, invalid: true } : suggestion,
        }),
      };
    },
    { initialProps: { enabled: true, scope: "revision1" } },
  );
  await waitFor(() => expect(result.current.smart.invalid).toBe(true));
  rerender({ enabled: true, scope: "revision2" });
  await waitFor(() => expect(result.current.options.important).toBe(true));
  expect(result.current.smart.invalid).toBe(false);
  rerender({ enabled: false, scope: "revision2" });
  await waitFor(() => expect(result.current.options.important).toBe(false));
});

it("clears stale automatic values immediately after a title change while keeping manual fields", async () => {
  const { result, rerender } = renderHook(
    ({ text }) => {
      const [options, setOptions] = useState(() => ({
        ...blankOptions(),
        reminder_at: "2026-10-05T12:00",
      }));
      return {
        options,
        smart: useSmartAdd({
          text,
          options,
          setOptions,
          enabled: true,
          defaultTime: "09:00",
          categories: [],
          ask: async () => ({ ...suggestion, deadline: "tomorrow" }),
        }),
      };
    },
    { initialProps: { text: "明日までに提出" } },
  );
  await waitFor(() => expect(result.current.options.important).toBe(true));
  rerender({ text: "別の用事" });
  expect(result.current.smart.currentOptions()).toMatchObject({
    important: false,
    deadline: "",
    reminder_at: "2026-10-05T12:00",
  });
});

it("keeps local duplicates after connection failure and resumes only after a new connection revision", async () => {
  let calls = 0;
  const { result, rerender } = renderHook(
    ({ text, scope }) => {
      const [options, setOptions] = useState(blankOptions);
      return useSmartAdd({
        text,
        scope,
        options,
        setOptions,
        enabled: true,
        defaultTime: "09:00",
        categories: [],
        existingTasks: [{ ...emptyTask(), id: 7, title: "既存の用事" }],
        ask: async () => {
          calls++;
          return {
            ...suggestion,
            important: false,
            unavailable: scope === "a",
          };
        },
      });
    },
    { initialProps: { text: "最初の用事", scope: "a" } },
  );
  await waitFor(() => expect(result.current.unavailable).toBe(true));
  rerender({ text: "既存の用事", scope: "a" });
  expect(result.current.duplicate?.task_id).toBe(7);
  await new Promise((r) => setTimeout(r, 500));
  expect(calls).toBe(1);
  rerender({ text: "既存の用事", scope: "b" });
  await waitFor(() => expect(calls).toBe(2));
});
