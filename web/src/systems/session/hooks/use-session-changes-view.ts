import { useQuery } from "@tanstack/react-query";
import { useEffect, useState, type RefObject } from "react";

import type { DiffViewerHandle } from "@compozy/ui";

import { copyFilePath, fileViewerTarget } from "@/systems/files";
import { useShellFocusedSessionId } from "@/systems/os";

import type { SessionChangesScope } from "../adapters/session-changes-api";
import { isLargeDiff, sessionChangeItems, type SessionChangeItem } from "../lib/session-changes";
import { sessionChangeBasesOptions } from "../lib/session-changes-query";
import { useSessionChanges, type SessionChangesModel } from "./use-session-changes";
import {
  useBaseSelectorOpen,
  useMissingBaseFallback,
  useSessionChangeSelection,
} from "./use-session-change-selection";
import { useSessionChangesLiveRefresh } from "./use-session-changes-live-refresh";
import {
  requestSessionReviewFile,
  type UseSessionReviewPanelStateResult,
} from "./use-session-review-panel-state";

/** Below this session-window width diffs are unified and the tree folds to its toggle. */
export const CHANGES_NARROW_WIDTH = 900;

function useNarrowSessionWindow(rootRef: RefObject<HTMLElement | null>): boolean {
  const [narrow, setNarrow] = useState(false);
  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof ResizeObserver === "undefined") return;
    const target = root.closest<HTMLElement>("[data-testid='session-window-split']") ?? root;
    const observer = new ResizeObserver(([entry]) => {
      if (entry) setNarrow(entry.contentRect.width < CHANGES_NARROW_WIDTH);
    });
    observer.observe(target);
    return () => observer.disconnect();
  }, [rootRef]);
  return narrow;
}

export interface SessionChangesView {
  changes: SessionChangesModel;
  items: SessionChangeItem[];
  /** The diff list's items: every file, or the current one in large mode. */
  shownItems: SessionChangeItem[];
  large: boolean;
  selectedPath: string | null;
  selectedIndex: number;
  scope: SessionChangesScope;
  branchAvailable: boolean;
  baseRef: string | undefined;
  /** A chosen base that no longer exists; Changes fell back to the default base. */
  missingBase: string | null;
  mode: "split" | "unified";
  narrow: boolean;
  treeOpen: boolean;
  allCollapsed: boolean;
  baseOpen: boolean;
  revealed: ReadonlySet<string>;
  setBaseOpen: (open: boolean) => void;
  select: (path: string) => void;
  step: (delta: -1 | 1) => void;
  onVisibleItemsChange: (ids: string[]) => void;
  toggleCollapsed: (item: SessionChangeItem, collapsed: boolean) => void;
  toggleCollapseAll: () => void;
  toggleTree: () => void;
  toggleMode: () => void;
  setScope: (scope: SessionChangesScope) => void;
  setBase: (ref: string) => void;
  reveal: (path: string) => void;
  openInViewer: (item: SessionChangeItem) => void;
  copyPath: (path: string) => void;
}

/**
 * The Changes tab's interaction state over its data: selection and the one-file stepper,
 * collapse and reveal choices kept across refreshes, scope/base/mode preferences, and a
 * changed-file click's focus request.
 */
export function useSessionChangesView({
  workspaceId,
  sessionId,
  review,
  rootRef,
  diffRef,
}: {
  workspaceId: string;
  sessionId: string;
  review: UseSessionReviewPanelStateResult;
  rootRef: RefObject<HTMLElement | null>;
  diffRef: RefObject<DiffViewerHandle | null>;
}): SessionChangesView {
  const [revealed, setRevealed] = useState<ReadonlySet<string>>(() => new Set());
  const [collapsed, setCollapsed] = useState<Readonly<Record<string, boolean>>>({});
  const [narrowTreeOpen, setNarrowTreeOpen] = useState(false);
  const narrow = useNarrowSessionWindow(rootRef);
  const changes = useSessionChanges({
    workspaceId,
    sessionId,
    ...(review.scope ? { scope: review.scope } : {}),
    ...(review.base ? { base: review.base } : {}),
    revealed,
    reloadItems: ids => ids.forEach(id => diffRef.current?.reloadItem(id)),
  });
  useSessionChangesLiveRefresh({
    workspaceId,
    sessionId,
    enabled: changes.summary !== undefined,
    windowFocused: useShellFocusedSessionId() === sessionId,
    onRefresh: changes.refresh,
  });
  const bases = useQuery(sessionChangeBasesOptions(workspaceId, sessionId, ""));
  const [baseOpen, setBaseOpen] = useBaseSelectorOpen(changes.errorCode);
  const { missingBase, dismissMissingBase } = useMissingBaseFallback(
    changes.errorCode,
    review.base,
    () => review.setChangesPreference({ base: undefined })
  );

  const { summary } = changes;
  const items = summary
    ? sessionChangeItems(summary, {
        revealed,
        collapsed,
        expandRefusals: changes.expandRefusals,
      })
    : [];
  const large = summary ? isLargeDiff(summary.totals) : false;
  const selection = useSessionChangeSelection({
    sessionId,
    summary,
    items,
    large,
    focus: review.focusedChange,
    diffRef,
  });
  const { selectedIndex, selectedPath } = selection;
  const shownItems = large ? items.slice(selectedIndex, selectedIndex + 1) : items;

  const allCollapsed = items.length > 0 && items.every(item => item.collapsed);
  const scope = summary?.scope === "uncommitted" ? "uncommitted" : (review.scope ?? "branch");
  return {
    changes,
    items,
    shownItems,
    large,
    selectedPath,
    selectedIndex,
    scope,
    branchAvailable: bases.data?.resolvable !== false,
    baseRef: summary?.base.ref || review.base,
    missingBase,
    mode: narrow ? "unified" : review.diffMode,
    narrow,
    treeOpen: narrow ? narrowTreeOpen : review.changesTree,
    allCollapsed,
    baseOpen,
    revealed,
    setBaseOpen,
    select: selection.select,
    step: selection.step,
    onVisibleItemsChange: selection.onVisibleItemsChange,
    toggleCollapsed: (item, current) => setCollapsed(prev => ({ ...prev, [item.path]: !current })),
    toggleCollapseAll: () =>
      setCollapsed(Object.fromEntries(items.map(item => [item.path, !allCollapsed]))),
    toggleTree: () =>
      narrow
        ? setNarrowTreeOpen(open => !open)
        : review.setChangesPreference({ changesTree: !review.changesTree }),
    toggleMode: () => {
      if (narrow) return;
      review.setChangesPreference({ diffMode: review.diffMode === "split" ? "unified" : "split" });
    },
    setScope: next => {
      dismissMissingBase();
      review.setChangesPreference({ scope: next });
    },
    setBase: ref => {
      dismissMissingBase();
      review.setChangesPreference({ base: ref, scope: "branch" });
    },
    reveal: path => setRevealed(prev => new Set(prev).add(path)),
    openInViewer: item => {
      if (item.file.status === "deleted") return;
      requestSessionReviewFile(
        sessionId,
        fileViewerTarget(item.path, changes.firstLineOf(item.id))
      );
    },
    copyPath: copyFilePath,
  };
}
