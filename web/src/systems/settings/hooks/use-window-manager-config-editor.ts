import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { findShortcutConflicts, windowManagerKeys, type WindowManagerConfig } from "@/systems/os";

import { updateWindowManagerSettings } from "../adapters/window-manager-layouts-api";
import { WINDOW_MANAGER_RANGES } from "../lib/window-manager-snap-geometry";

/**
 * Which value is out of range, not merely that one is. The old editor could only
 * disable Save and set `aria-invalid`, so a person with one bad number in a page
 * of controls had to hunt for it.
 */
export type WindowManagerConfigProblem =
  | { field: "gaps"; message: string }
  | { field: "snap"; message: string }
  | { field: "repeatRatios"; message: string }
  | { field: "historyLimit"; message: string }
  | { field: "shortcuts"; message: string };

function inRange(value: number, range: { min: number; max: number }): boolean {
  return Number.isInteger(value) && value >= range.min && value <= range.max;
}

function collectProblems(config: WindowManagerConfig): WindowManagerConfigProblem[] {
  const problems: WindowManagerConfigProblem[] = [];
  const ranges = WINDOW_MANAGER_RANGES;

  if (!Object.values(config.gaps).every(gap => inRange(gap, ranges.gap))) {
    problems.push({
      field: "gaps",
      message: `Gaps run from ${ranges.gap.min} to ${ranges.gap.max} pixels.`,
    });
  }

  const snapOutOfRange =
    !inRange(config.snap.edgeBand, ranges.edgeBand) ||
    !inRange(config.snap.cornerReach, ranges.cornerReach) ||
    !inRange(config.snap.exitSlack, ranges.exitSlack);
  if (snapOutOfRange) {
    problems.push({ field: "snap", message: "A snap threshold is outside its range." });
  }

  const ratios = config.snap.repeatRatios;
  const canonical = ratios.map(ratio => Math.round(ratio * 1_000_000));
  if (ratios.length < ranges.repeatStops.min || ratios.length > ranges.repeatStops.max) {
    problems.push({
      field: "repeatRatios",
      message: `Keep between ${ranges.repeatStops.min} and ${ranges.repeatStops.max} repeat widths.`,
    });
  } else if (new Set(canonical).size !== ratios.length) {
    problems.push({ field: "repeatRatios", message: "Two repeat widths sit at the same stop." });
  } else if (
    !ratios.every(ratio => ratio >= ranges.repeatRatio.min && ratio <= ranges.repeatRatio.max)
  ) {
    problems.push({
      field: "repeatRatios",
      message: "Repeat widths run from a tenth of the screen to nine tenths.",
    });
  }

  if (!inRange(config.historyLimit, ranges.historyLimit)) {
    problems.push({
      field: "historyLimit",
      message: `Layout history runs from ${ranges.historyLimit.min} to ${ranges.historyLimit.max} steps.`,
    });
  }

  // Two overrides on one chord is the case `CanonicalShortcuts` rejects; an
  // override landing on a shipped default is stored and merely shadows.
  const blocking = findShortcutConflicts(config.shortcuts).filter(
    conflict => conflict.kind === "override"
  );
  if (blocking.length > 0) {
    problems.push({
      field: "shortcuts",
      message: `${blocking.length} shortcut${blocking.length === 1 ? " is" : "s are"} assigned twice. The daemon refuses a duplicate chord.`,
    });
  }

  return problems;
}

export function useWindowManagerConfigEditor(baseline: WindowManagerConfig) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(baseline);
  const baselineKey = JSON.stringify(baseline);
  const [syncedKey, setSyncedKey] = useState(baselineKey);

  // The saved config is daemon state read through a query, so it can change
  // without this editor asking. Adopt it when nothing is in flight; keep the
  // edits otherwise. Remounting on every change threw them away instead.
  if (syncedKey !== baselineKey) {
    if (JSON.stringify(draft) === syncedKey) setDraft(baseline);
    setSyncedKey(baselineKey);
  }

  const dirty = JSON.stringify(draft) !== baselineKey;
  const problems = collectProblems(draft);

  const save = useMutation({
    mutationFn: async () => {
      await updateWindowManagerSettings(draft);
      return draft;
    },
    onSuccess: next => {
      queryClient.setQueryData(windowManagerKeys.config(), next);
    },
  });

  return {
    canSave: dirty && problems.length === 0 && !save.isPending,
    dirty,
    draft,
    error: save.error,
    isSaving: save.isPending,
    problems,
    reset: () => setDraft(baseline),
    save: () => save.mutate(),
    setDraft,
    setShortcuts: (shortcuts: Record<string, string>) =>
      setDraft(current => ({ ...current, shortcuts })),
  };
}

export type WindowManagerConfigEditorModel = ReturnType<typeof useWindowManagerConfigEditor>;
