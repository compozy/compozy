import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { windowManagerKeys } from "@/systems/os";

import { applyWindowManagerLayout } from "../adapters/window-manager-layouts-api";
import { parseWindowManagerLayoutDocument } from "../lib/window-manager-layout-schema";
import {
  windowManagerLayoutFingerprint,
  windowManagerLayoutReviewOptions,
} from "../lib/window-manager-layout-query";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutPreview,
  WindowManagerLayoutState,
  WindowManagerLayoutValidation,
  WindowManagerNormalizedRect,
} from "../lib/window-manager-layout-types";

interface ReviewedLayout {
  fingerprint: string;
  preview: WindowManagerLayoutPreview;
}

export function useWindowManagerLayoutEditor(
  workspaceId: string,
  initial: WindowManagerLayoutState
) {
  const queryClient = useQueryClient();
  // The daemon's own state, not a copy of it: the layout query shares the OS
  // snapshot cache, which the window-manager stream writes whenever any client
  // moves a window. Snapshotting it into local state (or remounting on every
  // revision) is what used to throw the draft away mid-gesture.
  const { revision, document: baseline } = initial;
  const [draft, setDraft] = useState(baseline);
  const [importError, setImportError] = useState<string | null>(null);
  const baselineFingerprint = windowManagerLayoutFingerprint(baseline);
  const currentFingerprint = windowManagerLayoutFingerprint(draft);
  const [syncedFingerprint, setSyncedFingerprint] = useState(baselineFingerprint);

  if (syncedFingerprint !== baselineFingerprint) {
    // The workspace moved underneath the editor. Adopt it when there is nothing
    // to lose; otherwise keep the edits — the review query is keyed by revision,
    // so Apply re-arms only after the daemon has checked them against the new one.
    if (currentFingerprint === syncedFingerprint) setDraft(baseline);
    setSyncedFingerprint(baselineFingerprint);
  }

  const dirty = currentFingerprint !== baselineFingerprint;

  const updateDraft = (next: WindowManagerLayoutDocument) => {
    setDraft(next);
  };

  const review = useQuery(windowManagerLayoutReviewOptions(workspaceId, revision, draft));
  const reviewResult = review.data?.fingerprint === currentFingerprint ? review.data : null;
  const validation: WindowManagerLayoutValidation | null = reviewResult?.validation ?? null;
  const reviewed: ReviewedLayout | null =
    reviewResult?.preview == null
      ? null
      : { fingerprint: reviewResult.fingerprint, preview: reviewResult.preview };
  const reviewCurrent = reviewed?.fingerprint === currentFingerprint;

  const apply = useMutation({
    mutationFn: async () => {
      const candidate = structuredClone(draft);
      const result = await applyWindowManagerLayout(workspaceId, revision, candidate);
      return { candidate, result };
    },
    // Refetching is the whole reconciliation: the snapshot comes back at the new
    // revision carrying the applied document, so the baseline and the draft meet
    // and `dirty` falls to false without a second source of truth.
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: windowManagerKeys.snapshot(workspaceId),
      });
    },
  });

  const importDocument = async (file: File) => {
    try {
      const value: unknown = JSON.parse(await file.text());
      const imported = parseWindowManagerLayoutDocument(value);
      updateDraft({ ...imported, workspaceId });
      setImportError(null);
    } catch (error) {
      setImportError(
        error instanceof Error ? error.message : "The selected file is not a layout document."
      );
    }
  };

  /** Whole-desktop swap: the canvas already produced an immutable next desktop. */
  const setDesktop = (index: number, next: WindowManagerLayoutDesktop) => {
    updateDraft({
      ...draft,
      desktops: draft.desktops.map((desktop, position) => (position === index ? next : desktop)),
    });
  };

  const setDesktopName = (index: number, name: string) => {
    const desktop = draft.desktops[index];
    if (!desktop) return;
    setDesktop(index, { ...desktop, name });
  };

  const setFloatingRect = (windowId: string, floatingRect: WindowManagerNormalizedRect) => {
    const window = draft.windows[windowId];
    if (!window) return;
    updateDraft({
      ...draft,
      windows: { ...draft.windows, [windowId]: { ...window, floatingRect } },
    });
  };

  return {
    apply,
    dirty,
    draft,
    exportDocument: () => structuredClone(draft),
    importDocument,
    importError,
    mutationError: review.error ?? apply.error,
    reset: () => updateDraft(structuredClone(baseline)),
    review,
    reviewed,
    reviewCurrent,
    revision,
    setDesktop,
    setDesktopName,
    setFloatingRect,
    updateDraft,
    validation,
  };
}

export type WindowManagerLayoutEditorModel = ReturnType<typeof useWindowManagerLayoutEditor>;
