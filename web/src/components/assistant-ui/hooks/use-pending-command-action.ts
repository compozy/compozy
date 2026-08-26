import { useEffect, useRef } from "react";
import { useAui } from "@assistant-ui/react";

function removeSelectedActionToken(beforeSelection: string, text: string, token: string): string {
  let changedStart = 0;
  while (
    changedStart < beforeSelection.length &&
    changedStart < text.length &&
    beforeSelection[changedStart] === text[changedStart]
  ) {
    changedStart += 1;
  }

  let unchangedSuffix = 0;
  while (
    unchangedSuffix < beforeSelection.length - changedStart &&
    unchangedSuffix < text.length - changedStart &&
    beforeSelection[beforeSelection.length - unchangedSuffix - 1] ===
      text[text.length - unchangedSuffix - 1]
  ) {
    unchangedSuffix += 1;
  }

  const changedEnd = text.length - unchangedSuffix;
  const candidates: number[] = [];
  let searchFrom = 0;
  while (searchFrom <= text.length - token.length) {
    const candidate = text.indexOf(token, searchFrom);
    if (candidate < 0) break;
    if (candidate < changedEnd && candidate + token.length > changedStart) {
      candidates.push(candidate);
    }
    searchFrom = candidate + token.length;
  }
  const at = candidates.sort(
    (left, right) => Math.abs(left - changedStart) - Math.abs(right - changedStart)
  )[0];
  if (at === undefined) {
    const triggerStart = beforeSelection.search(/\S/);
    if (text !== beforeSelection || triggerStart < 0 || text[triggerStart] !== "/") return text;
    let triggerEnd = triggerStart + 1;
    while (triggerEnd < text.length && !/\s/.test(text[triggerEnd]!)) triggerEnd += 1;
    return `${text.slice(0, triggerStart)}${text.slice(triggerEnd)}`;
  }
  const before = text.slice(0, at);
  const after = text.slice(at + token.length);
  return `${before}${before.endsWith(" ") && after.startsWith(" ") ? after.slice(1) : after}`;
}

export function usePendingCommandAction(composerText: string) {
  const aui = useAui();
  const pendingActionRef = useRef<{ beforeSelection: string; token: string } | null>(null);

  useEffect(() => {
    const pendingAction = pendingActionRef.current;
    if (!pendingAction) return;
    pendingActionRef.current = null;
    const nextText = removeSelectedActionToken(
      pendingAction.beforeSelection,
      composerText,
      pendingAction.token
    );
    if (nextText === composerText) return;
    aui.composer.setText(nextText);
  }, [aui, composerText]);

  return pendingActionRef;
}
