import type { PillTone } from "@compozy/ui";

import type {
  SettingsUpdateOperation,
  SettingsUpdateStatus,
  SettingsUpdateStatusKind,
  SettingsUpdateTarget,
  SettingsUpdateTargetSet,
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
 * and never mutate, so they never qualify. A failed track remains diagnostic-only
 * until the daemon reports a fresh available state; PlanOperation rejects failed
 * targets even when the lease is free.
 */
function canApplyTrack(input: {
  status: SettingsUpdateStatusKind;
  managed: boolean;
  operation: SettingsUpdateOperation | null;
  latestVersion: string | null;
}): boolean {
  if (input.managed || input.operation !== null || input.latestVersion === null) return false;
  return input.status === "available";
}

function dormantOperationTarget(
  operation: SettingsUpdateOperation | null
): SettingsUpdateTarget | null {
  if (!operation || operation.holder !== null) return null;
  if (operation.waiting === "waiting-for-app" && operation.targets.includes("app")) return "app";
  return null;
}

/** The dormant operation, if any, belongs to exactly one waiting target. */
function canCancelTrack(
  operation: SettingsUpdateOperation | null,
  target: SettingsUpdateTarget
): boolean {
  return dormantOperationTarget(operation) === target;
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

interface SettingsUpdateTrackSource {
  status: SettingsUpdateStatusKind;
  current_version: string;
  latest_version?: string;
  release_url?: string;
  message: string;
  last_error?: string;
}

function buildTrackView(input: {
  id: SettingsUpdateTarget;
  label: string;
  source: SettingsUpdateTrackSource;
  managed: boolean;
  recommendation: string | null;
  restoredVersion: string | null;
  operation: SettingsUpdateOperation | null;
}): SettingsUpdateTrackView {
  const latestVersion = text(input.source.latest_version);
  const canApply = canApplyTrack({
    status: input.source.status,
    managed: input.managed,
    operation: input.operation,
    latestVersion,
  });

  return {
    id: input.id,
    label: input.label,
    status: input.source.status,
    tone: settingsUpdateStatusTone(input.source.status),
    statusLabel: settingsUpdateStatusLabel(input.source.status),
    currentVersion: input.source.current_version,
    latestVersion,
    releaseUrl: text(input.source.release_url),
    recommendation: input.recommendation,
    restoredVersion: input.restoredVersion,
    message: input.source.message,
    description: trackDescription({
      status: input.source.status,
      canApply,
      message: input.source.message,
    }),
    lastError: text(input.source.last_error),
    canApply,
    canCancel: canCancelTrack(input.operation, input.id),
    progress: trackProgress(input.operation, input.id, input.source.status),
  };
}

/**
 * Ordered track render models. The runtime track always exists; the app track
 * exists exactly when a desktop app is installed on this host, so an absent app
 * yields a single-track section rather than an empty row.
 */
export function settingsUpdateTracks(snapshot: SettingsUpdateStatus): SettingsUpdateTrackView[] {
  const operation = snapshot.operation ?? null;
  const runtime = snapshot.runtime;
  const tracks: SettingsUpdateTrackView[] = [
    buildTrackView({
      id: "runtime",
      label: "Runtime",
      source: runtime,
      managed: runtime.managed,
      recommendation: text(runtime.recommendation),
      restoredVersion: text(runtime.restored_version),
      operation,
    }),
  ];
  const app = snapshot.app;
  if (app) {
    tracks.push(
      buildTrackView({
        id: "app",
        label: "App",
        source: app,
        managed: false,
        recommendation: null,
        restoredVersion: null,
        operation,
      })
    );
  }
  return tracks;
}

/** The ordered targets this surface may apply in one durable operation. */
export function settingsUpdateApplicableTargets(
  tracks: readonly Pick<SettingsUpdateTrackView, "id" | "canApply">[]
): SettingsUpdateTargetSet | null {
  let runtime = false;
  let app = false;
  for (const track of tracks) {
    if (!track.canApply) continue;
    if (track.id === "runtime") runtime = true;
    if (track.id === "app") app = true;
  }
  if (runtime && app) return ["runtime", "app"];
  if (runtime) return ["runtime"];
  if (app) return ["app"];
  return null;
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
