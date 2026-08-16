import type { PillTone } from "@compozy/ui";

import type {
  SettingsUpdateOperation,
  SettingsUpdateStatus,
  SettingsUpdateStatusKind,
  SettingsUpdateTarget,
} from "../types";
import { updatePhasePercent, updateUiPhase, type UpdateUiPhase } from "./update-phase-map";

export type SettingsUpdateView =
  | { kind: "checking" }
  | { kind: "error"; message: string }
  | { kind: "unavailable"; message: string }
  | { kind: "snapshot"; snapshot: SettingsUpdateStatus; refreshError: string | null };

/**
 * Live progress for the one track an operation is currently acting on. Present
 * only when the journaled phase has a measurable UI edge, so the phase is always
 * a real one — an unmapped phase yields no progress at all rather than a blank.
 */
export interface SettingsUpdateProgress {
  phase: UpdateUiPhase;
  /** `null` when the phase reports no measurable percent. */
  percent: number | null;
}

/** One track's render model. Every field is daemon truth or `null` (SD-007). */
export interface SettingsUpdateTrackView {
  id: SettingsUpdateTarget;
  label: string;
  status: SettingsUpdateStatusKind;
  tone: PillTone;
  statusLabel: string;
  currentVersion: string;
  latestVersion: string | null;
  releaseUrl: string | null;
  /** Managed installs only: the exact upgrade command, shown verbatim. */
  recommendation: string | null;
  /** Set when the last apply rolled back to this version. */
  restoredVersion: string | null;
  message: string;
  /**
   * The daemon message, or `null` when it would only restate the version and
   * pill — a settled or plainly offered track says everything in its own lane.
   */
  description: string | null;
  lastError: string | null;
  /** False ⇒ the apply affordance is absent from the DOM, never disabled. */
  canApply: boolean;
  /** True only on the track holding a dormant operation this surface may cancel. */
  canCancel: boolean;
  progress: SettingsUpdateProgress | null;
}

const STATUS_TONES: Record<SettingsUpdateStatusKind, PillTone> = {
  "up-to-date": "success",
  updated: "success",
  available: "info",
  accepted: "info",
  applying: "info",
  staged: "info",
  blocked: "warning",
  failed: "danger",
  unsupported: "neutral",
  canceled: "neutral",
};

const STATUS_LABELS: Record<SettingsUpdateStatusKind, string> = {
  "up-to-date": "Up to date",
  updated: "Updated",
  available: "Update available",
  accepted: "Starting",
  applying: "Updating",
  staged: "Staged",
  blocked: "Blocked",
  failed: "Update failed",
  unsupported: "Unsupported",
  canceled: "Canceled",
};

export function settingsUpdateStatusTone(status: SettingsUpdateStatusKind): PillTone {
  return STATUS_TONES[status];
}

export function settingsUpdateStatusLabel(status: SettingsUpdateStatusKind): string {
  return STATUS_LABELS[status];
}

function updateErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Update check failed";
}

function text(value: string | undefined | null): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
}

export function settingsUpdateView(input: {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  data?: SettingsUpdateStatus;
}): SettingsUpdateView {
  if (input.data) {
    return {
      kind: "snapshot",
      snapshot: input.data,
      refreshError: input.isError ? updateErrorMessage(input.error) : null,
    };
  }
  if (input.isLoading) return { kind: "checking" };
  if (input.isError) {
    return { kind: "error", message: updateErrorMessage(input.error) };
  }
  return { kind: "unavailable", message: "Update status is unavailable." };
}

/**
 * Live progress belongs to a track only while that track is itself mid-flight.
 *
 * A `blocked` track is not being updated — it is being refused, and the operation
 * whose phase the payload reports belongs to whoever holds the lease. Showing
 * their download against our refused request would claim work we are not doing.
 * A dormant record (a staged app) reports a phase with no measurable edge, so it
 * keeps its status pill instead.
 */
function trackProgress(
  operation: SettingsUpdateOperation | null,
  target: SettingsUpdateTarget,
  status: SettingsUpdateStatusKind
): SettingsUpdateProgress | null {
  if (!operation) return null;
  if (operation.active_target !== target) return null;
  if (status !== "applying" && status !== "accepted") return null;
  const phase = updateUiPhase(target, operation.phase, operation.percent);
  if (!phase) return null;
  return { phase, percent: updatePhasePercent(operation.percent) };
}

/**
 * Whether this track can be applied from here.
 *
 * Requires something to apply, a self-applicable install, and a free update
 * channel — acquisition is single-flight per home, so a live operation makes
 * apply impossible rather than merely inadvisable. Managed installs recommend
 * and never mutate, so they never qualify. A `failed` track qualifies again once
 * its operation is terminal: the lease is free and the offer still stands.
 */
function canApplyTrack(input: {
  status: SettingsUpdateStatusKind;
  managed: boolean;
  operation: SettingsUpdateOperation | null;
}): boolean {
  if (input.managed || input.operation !== null) return false;
  return input.status === "available" || input.status === "failed";
}

/** The dormant operation, if any, belongs to exactly one track: its active target. */
function canCancelTrack(
  operation: SettingsUpdateOperation | null,
  target: SettingsUpdateTarget
): boolean {
  if (!operation) return false;
  return (
    operation.holder === null && operation.waiting !== "" && operation.active_target === target
  );
}

/**
 * The message earns a row description only where it adds truth. A settled track,
 * or one plainly offering an update this surface can apply, already says
 * everything through its version lane and pill; repeating it there would be prose
 * for its own sake.
 */
function trackDescription(input: {
  status: SettingsUpdateStatusKind;
  canApply: boolean;
  message: string;
}): string | null {
  if (input.status === "up-to-date") return null;
  if (input.status === "available" && input.canApply) return null;
  return text(input.message);
}

/**
 * Ordered track render models. The runtime track always exists; the app track
 * exists exactly when a desktop app is installed on this host, so an absent app
 * yields a single-track section rather than an empty row.
 */
export function settingsUpdateTracks(snapshot: SettingsUpdateStatus): SettingsUpdateTrackView[] {
  const operation = snapshot.operation ?? null;
  const runtime = snapshot.runtime;
  const runtimeCanApply = canApplyTrack({
    status: runtime.status,
    managed: runtime.managed,
    operation,
  });
  const tracks: SettingsUpdateTrackView[] = [
    {
      id: "runtime",
      label: "Runtime",
      status: runtime.status,
      tone: settingsUpdateStatusTone(runtime.status),
      statusLabel: settingsUpdateStatusLabel(runtime.status),
      currentVersion: runtime.current_version,
      latestVersion: text(runtime.latest_version),
      releaseUrl: text(runtime.release_url),
      recommendation: text(runtime.recommendation),
      restoredVersion: text(runtime.restored_version),
      message: runtime.message,
      description: trackDescription({
        status: runtime.status,
        canApply: runtimeCanApply,
        message: runtime.message,
      }),
      lastError: text(runtime.last_error),
      canApply: runtimeCanApply,
      canCancel: canCancelTrack(operation, "runtime"),
      progress: trackProgress(operation, "runtime", runtime.status),
    },
  ];
  const app = snapshot.app;
  if (app) {
    const appCanApply = canApplyTrack({ status: app.status, managed: false, operation });
    tracks.push({
      id: "app",
      label: "App",
      status: app.status,
      tone: settingsUpdateStatusTone(app.status),
      statusLabel: settingsUpdateStatusLabel(app.status),
      currentVersion: app.current_version,
      latestVersion: text(app.latest_version),
      releaseUrl: text(app.release_url),
      recommendation: null,
      restoredVersion: null,
      message: app.message,
      description: trackDescription({
        status: app.status,
        canApply: appCanApply,
        message: app.message,
      }),
      lastError: text(app.last_error),
      canApply: appCanApply,
      canCancel: canCancelTrack(operation, "app"),
      progress: trackProgress(operation, "app", app.status),
    });
  }
  return tracks;
}

/**
 * The version lane: a bare current version once nothing is pending, or the
 * `current → latest` transition while a different build is offered or targeted.
 */
export function settingsUpdateVersionTransition(
  track: Pick<SettingsUpdateTrackView, "currentVersion" | "latestVersion">
): string | null {
  if (!track.latestVersion || track.latestVersion === track.currentVersion) return null;
  return track.latestVersion;
}

/**
 * Whether the dormant-operation cancel affordance applies.
 *
 * Cancel frees a record nobody is executing — a `waiting-for-app` operation or an
 * expired lease. A live holder owns the record, so cancel is not offered for it
 * (ADR-009 B-013); the daemon would decline anyway.
 */
export function settingsUpdateCancelable(snapshot: SettingsUpdateStatus): boolean {
  const operation = snapshot.operation;
  if (!operation) return false;
  return operation.holder === null && operation.waiting !== "";
}

/**
 * Whether the menubar indicator should exist at all.
 *
 * True only while an update is genuinely offered and nothing is running: a live
 * operation suppresses the indicator outright, because applying, staged, and
 * failed are Settings' job and the menubar never renders progress or errors
 * (`_uiux.md` S2, US-029.EC-4).
 */
export function settingsUpdateIndicatorAvailable(
  snapshot: SettingsUpdateStatus | undefined
): boolean {
  if (!snapshot) return false;
  if (snapshot.operation) return false;
  return snapshot.runtime.status === "available" || snapshot.app?.status === "available";
}
