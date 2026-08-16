import type { SettingsUpdateTarget } from "../types";

/**
 * The fixed UI phase vocabulary an update operation can report. Closed and
 * exhaustive by contract (`_spec.md` Part II, phase→UI mapping): the surface
 * renders these names verbatim and invents nothing per-surface.
 */
export const UPDATE_UI_PHASES = [
  "download",
  "verify",
  "install",
  "start",
  "ready-check",
  "ready",
] as const;

export type UpdateUiPhase = (typeof UPDATE_UI_PHASES)[number];

const RUNTIME_PHASES: Record<string, UpdateUiPhase> = {
  downloading: "download",
  verifying: "verify",
  swapping: "install",
  restarting: "start",
  "health-checking": "ready-check",
  finalized: "ready",
};

const APP_PHASES: Record<string, UpdateUiPhase> = {
  "installer-handoff": "install",
  restarted: "start",
  verified: "ready",
};

/**
 * Maps a journaled operation phase onto the fixed UI vocabulary.
 *
 * Mirrors the Go authority `compozyupdate.PhaseForUI` (`internal/update/projection.go`)
 * edge for edge. The settings projection ships the raw journal phase rather than
 * a pre-mapped one, so the mapping is applied here at the render boundary.
 *
 * Phases with no measurable UI edge (`pending`, `staged`, `rolled-back`, `failed`)
 * and any unknown phase return `null` — the surface then renders the track's own
 * status rather than inventing a progress step (SD-007).
 */
export function updateUiPhase(
  target: SettingsUpdateTarget | undefined,
  phase: string | undefined,
  percent: number
): UpdateUiPhase | null {
  if (!phase) return null;
  if (target === "runtime") return RUNTIME_PHASES[phase] ?? null;
  if (target === "app") {
    // The app updater reports one `applying` phase whose progress events carry
    // the download; a completed transfer is the verify edge.
    if (phase === "applying") return percent >= 100 ? "verify" : "download";
    return APP_PHASES[phase] ?? null;
  }
  return null;
}

/** Percent is `-1` whenever the active phase has no measurable progress. */
export function updatePhasePercent(percent: number): number | null {
  return percent >= 0 && percent <= 100 ? percent : null;
}
