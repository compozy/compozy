import { useEffect, useEffectEvent } from "react";

export type ShortcutAction =
  | "next"
  | "previous"
  | "next-state"
  | "previous-state"
  | "theme"
  | "comment"
  | "search"
  | "help"
  | "sidebar";

const keys: Record<string, ShortcutAction> = {
  j: "next",
  k: "previous",
  "]": "next-state",
  "[": "previous-state",
  t: "theme",
  c: "comment",
  "/": "search",
  "?": "help",
};

function typing(target: EventTarget | null) {
  if (!target || !("closest" in target)) return false;
  return Boolean(
    (target as Element).closest(
      "input, textarea, select, [contenteditable=''], [contenteditable='true']"
    )
  );
}

/**
 * Gallery keyboard shortcuts. They listen on the gallery and inside the canvas iframe (same
 * origin), and stay quiet while typing in a field or with a modifier held, except Cmd/Ctrl+B.
 */
export function useShortcuts(onAction: (action: ShortcutAction) => void, frame: Window | null) {
  const handle = useEffectEvent((event: KeyboardEvent) => {
    if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "b") {
      event.preventDefault();
      onAction("sidebar");
      return;
    }
    if (event.metaKey || event.ctrlKey || event.altKey || event.defaultPrevented) return;
    if (typing(event.target)) return;
    const action = keys[event.key.length === 1 ? event.key.toLowerCase() : event.key];
    if (!action) return;
    event.preventDefault();
    onAction(action);
  });

  useEffect(() => {
    const targets = frame ? [window, frame] : [window];
    const listener = (event: Event) => handle(event as KeyboardEvent);
    for (const target of targets) target.addEventListener("keydown", listener);
    return () => {
      for (const target of targets) target.removeEventListener("keydown", listener);
    };
  }, [frame]);
}
