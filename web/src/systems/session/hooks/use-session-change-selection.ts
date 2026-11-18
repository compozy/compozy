import { useEffect, useEffectEvent, useRef, useState, type RefObject } from "react";

import type { DiffViewerHandle } from "@compozy/ui";

import { fileViewerTarget } from "@/systems/files";

import type { OpenInFilesApp } from "./use-open-in-files-app";

import type { SessionChangesSummary } from "../adapters/session-changes-api";
import { focusedChangeRoute, type SessionChangeItem } from "../lib/session-changes";
import type { SessionChangeFocus } from "./use-session-review-panel-state";

/** The base selector's open state; a chosen base that no longer exists reopens it (US-008.EC-1). */
export function useBaseSelectorOpen(errorCode: string | null) {
  const [open, setOpen] = useState(false);
  const missing = errorCode === "base_not_found";
  const [missingSeen, setMissingSeen] = useState(false);
  if (missing !== missingSeen) {
    setMissingSeen(missing);
    if (missing) setOpen(true);
  }
  return [open, setOpen] as const;
}

/**
 * A persisted base that no longer exists falls back to the default base instead of staying
 * persisted, so focus refreshes stop asking for it (US-008.EC-1). The lost ref stays named
 * until the operator picks a base or scope.
 */
export function useMissingBaseFallback(
  errorCode: string | null,
  base: string | undefined,
  clearBase: () => void
) {
  const missing = errorCode === "base_not_found" && base ? base : null;
  const [lost, setLost] = useState<string | null>(null);
  if (missing !== null && lost !== missing) setLost(missing);
  const clearBaseRef = useRef(clearBase);
  useEffect(() => {
    clearBaseRef.current = clearBase;
  });
  useEffect(() => {
    if (missing !== null) clearBaseRef.current();
  }, [missing]);
  return { missingBase: lost, dismissMissingBase: () => setLost(null) };
}

/**
 * The selected changed file: tree clicks, `[`/`]` steps, scrolling the diffs, and a
 * changed-file row's focus request (US-005.AC-6), which opens the Files window instead when
 * the summary doesn't list the file. A file that leaves the summary hands its selection to the
 * file now at its place (US-010.EC-1).
 */
export function useSessionChangeSelection({
  summary,
  items,
  large,
  focus,
  diffRef,
  openInFiles,
}: {
  summary: SessionChangesSummary | undefined;
  items: readonly SessionChangeItem[];
  large: boolean;
  focus: SessionChangeFocus | null;
  diffRef: RefObject<DiffViewerHandle | null>;
  openInFiles: OpenInFilesApp | null;
}) {
  const [selection, setSelection] = useState<{ path: string | null; index: number }>({
    path: null,
    index: 0,
  });
  const listedIndex = items.findIndex(item => item.path === selection.path);
  const selectedIndex =
    listedIndex >= 0 ? listedIndex : Math.min(selection.index, Math.max(0, items.length - 1));
  const selectedPath = items[selectedIndex]?.path ?? null;

  const select = (path: string) => {
    const index = items.findIndex(item => item.path === path);
    if (index < 0) return;
    setSelection({ path, index });
    if (!large) diffRef.current?.scrollToItem(path);
  };

  // Applied once per request, as soon as a summary can say whether the file is listed.
  const [appliedFocus, setAppliedFocus] = useState<number | null>(null);
  if (focus && summary && appliedFocus !== focus.nonce) {
    setAppliedFocus(focus.nonce);
    const index = summary.files.findIndex(file => file.path === focus.path);
    if (index >= 0) setSelection({ path: focus.path, index });
  }
  const handledFocus = useRef<number | null>(null);
  // An event, not a dependency: the owner's callback is fresh every render, and re-running the
  // effect would cancel the pending scroll of a focus that is already handled.
  const openUnlisted = useEffectEvent((path: string) => openInFiles?.(fileViewerTarget(path)));
  useEffect(() => {
    if (!focus || !summary || appliedFocus !== focus.nonce) return;
    if (handledFocus.current === focus.nonce) return;
    handledFocus.current = focus.nonce;
    if (focusedChangeRoute(summary.files, focus.path) === "files") {
      openUnlisted(focus.path);
      return;
    }
    if (large) return;
    // The diff list renders the new selection first; scroll on the next frame.
    const frame = requestAnimationFrame(() => diffRef.current?.scrollToItem(focus.path));
    return () => cancelAnimationFrame(frame);
  }, [appliedFocus, diffRef, focus, large, summary]);

  return {
    selectedIndex,
    selectedPath,
    select,
    step: (delta: -1 | 1) => {
      const next = items[selectedIndex + delta];
      if (next) select(next.path);
    },
    onVisibleItemsChange: (ids: string[]) => {
      if (large || ids.length === 0 || (selectedPath && ids.includes(selectedPath))) return;
      const first = ids[0]!;
      setSelection({ path: first, index: items.findIndex(item => item.path === first) });
    },
  };
}
