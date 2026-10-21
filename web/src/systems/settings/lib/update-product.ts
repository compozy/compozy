import type { PillTone } from "@compozy/ui";

import type {
  SettingsUpdateHolder,
  SettingsUpdateStatus,
  SettingsUpdateStatusKind,
  SettingsUpdateTargetSet,
} from "../types";
import {
  settingsUpdateApplicableTargets,
  settingsUpdateStatusLabel,
  settingsUpdateStatusTone,
  settingsUpdateTracks,
  type SettingsUpdateTrackView,
} from "./update-presentation";

/**
 * One product update, as the person using CompozyOS sees it.
 *
 * The daemon keeps two tracks (runtime and desktop app) and applies them as one
 * durable operation; that split is plumbing. This view folds both tracks into a
 * single status, version, and action so no surface ever asks the user to reason
 * about which part of the product is updating.
 */
export interface SettingsProductUpdateView {
  status: SettingsUpdateStatusKind;
  tone: PillTone;
  statusLabel: string;
  /** The product version the daemon runs today. */
  currentVersion: string;
  /** The version on offer, or `null` when nothing newer is pending. */
  latestVersion: string | null;
  releaseUrl: string | null;
  /** One plain sentence where the state needs explaining, else `null`. */
  description: string | null;
  /** The manual upgrade command for any part still behind, shown verbatim. */
  recommendation: string | null;
  /** Everything this surface may apply in one click, or `null` when it may not. */
  targets: SettingsUpdateTargetSet | null;
  /** True while an operation is actively running (not a dormant staged record). */
  updating: boolean;
  /** Measured percent of the step currently running, when one is measured. */
  percent: number | null;
  /** The raw UI phase of the running step, kept for diagnostics (`data-phase`). */
  phase: string | null;
  canCancel: boolean;
  lastError: string | null;
}

/** What the menubar renders: nothing, an offer, live work, or a failure to look at. */
export type ProductUpdateIndicatorState = "hidden" | "available" | "updating" | "failed";

const HOLDER_SURFACE_LABEL: Record<SettingsUpdateHolder["surface"], string> = {
  cli: "the command line",
  daemon: "CompozyOS",
  web: "another browser window",
  shell: "the desktop app",
};

export function settingsUpdateHolderLabel(holder: SettingsUpdateHolder): string {
  return HOLDER_SURFACE_LABEL[holder.surface];
}

/**
 * The upgrade the user must run by hand, if any. Only a runtime this surface
 * cannot update owes one: a managed install with a newer release, or an install
 * method that cannot self-update. A self-updating runtime's "Run `compozy update`"
 * is what the Update button already does, so it never shows here. The manual
 * path stays visible even when another part can update itself from here.
 */
function manualRecommendation(snapshot: SettingsUpdateStatus): string | null {
  const runtime = snapshot.runtime;
  const manual = runtime.managed
    ? runtime.status === "available"
    : runtime.status === "unsupported";
  return manual ? text(runtime.recommendation) : null;
}

function text(value: string | undefined | null): string | null {
  const trimmed = value?.trim();
  return trimmed ? trimmed : null;
}

function liveTrack(tracks: SettingsUpdateTrackView[]): SettingsUpdateTrackView | null {
  return tracks.find(track => track.status === "applying" || track.status === "accepted") ?? null;
}

function pendingVersion(tracks: SettingsUpdateTrackView[], currentVersion: string): string | null {
  for (const track of tracks) {
    if (track.status === "up-to-date" || track.status === "unsupported") continue;
    if (track.latestVersion && track.latestVersion !== currentVersion) return track.latestVersion;
  }
  return null;
}

function productStatus(
  snapshot: SettingsUpdateStatus,
  targets: SettingsUpdateTargetSet | null
): SettingsUpdateStatusKind {
  if (targets?.some(target => target === "app") && snapshot.app?.status === "failed")
    return "failed";
  // Something applicable is the user's next step even when another track
  // reports a settled or failed outcome; the footnotes keep that history.
  if (targets) return "available";
  return snapshot.aggregate;
}

function productDescription(input: {
  status: SettingsUpdateStatusKind;
  snapshot: SettingsUpdateStatus;
  tracks: SettingsUpdateTrackView[];
  targets: SettingsUpdateTargetSet | null;
  recommendation: string | null;
}): string | null {
  const { status, snapshot, tracks } = input;
  switch (status) {
    case "available":
      if (!input.recommendation) return null;
      return input.targets
        ? "Update installs what it can here; do the rest by hand with:"
        : "Update with your package manager to get the new version.";
    case "staged":
      return "The update finishes the next time you open the CompozyOS app.";
    case "blocked": {
      const holder = snapshot.operation?.holder;
      return holder
        ? `An update is already running from ${settingsUpdateHolderLabel(holder)}.`
        : "An update is already running.";
    }
    case "failed": {
      const restored = tracks.find(track => track.restoredVersion)?.restoredVersion;
      return restored
        ? `The update didn't finish, so CompozyOS went back to ${restored}.`
        : "The update didn't finish.";
    }
    default:
      // An install this surface cannot update (unsupported platform, managed
      // runtime) still owes the user its manual path.
      return input.recommendation ? "Update this install manually with:" : null;
  }
}

/** Folds the daemon's per-track snapshot into the one update the user sees. */
export function settingsProductUpdate(snapshot: SettingsUpdateStatus): SettingsProductUpdateView {
  const tracks = settingsUpdateTracks(snapshot);
  const targets = settingsUpdateApplicableTargets(tracks);
  const status = productStatus(snapshot, targets);
  const currentVersion = snapshot.runtime.current_version;
  const live = liveTrack(tracks);
  const recommendation = manualRecommendation(snapshot);
  const operation = snapshot.operation ?? null;

  return {
    status,
    tone: settingsUpdateStatusTone(status),
    statusLabel: settingsUpdateStatusLabel(status),
    currentVersion,
    latestVersion: pendingVersion(tracks, currentVersion),
    releaseUrl: tracks.find(track => track.releaseUrl)?.releaseUrl ?? null,
    description: productDescription({ status, snapshot, tracks, targets, recommendation }),
    recommendation,
    targets,
    updating: operation !== null && (operation.holder !== null || live !== null),
    percent: live?.progress?.percent ?? null,
    phase: live?.progress?.phase ?? null,
    canCancel: tracks.some(track => track.canCancel),
    lastError: tracks.find(track => track.lastError)?.lastError ?? null,
  };
}

/**
 * The menubar's state for one snapshot. `applying` covers the moment between the
 * click and the daemon journaling the operation, so the control never flickers
 * back to an offer while the request is still in flight.
 */
export function settingsUpdateIndicatorState(
  snapshot: SettingsUpdateStatus | undefined,
  applying: boolean
): ProductUpdateIndicatorState {
  if (applying) return "updating";
  if (!snapshot) return "hidden";
  const product = settingsProductUpdate(snapshot);
  if (product.updating) return "updating";
  if (product.status === "available") return "available";
  if (product.status === "failed") return "failed";
  return "hidden";
}
