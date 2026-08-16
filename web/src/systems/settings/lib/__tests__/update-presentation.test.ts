import { describe, expect, it } from "vitest";

import {
  settingsUpdateApplyingFixture,
  settingsUpdateBlockedFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateManagedFixture,
  settingsUpdateNoAppFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateStagedFixture,
  settingsUpdateStatusFixture,
} from "../../mocks/settings-update-fixture";
import {
  settingsUpdateCancelable,
  settingsUpdateIndicatorAvailable,
  settingsUpdateTracks,
  settingsUpdateView,
} from "../update-presentation";
import { updatePhasePercent, updateUiPhase } from "../update-phase-map";

const snapshot = settingsUpdateBothAvailableFixture;

describe("settingsUpdateView", () => {
  it("Should project checking, unavailable, and initial error states without inventing install data", () => {
    expect(settingsUpdateView({ isLoading: true, isError: false, error: null })).toEqual({
      kind: "checking",
    });
    expect(settingsUpdateView({ isLoading: false, isError: false, error: null })).toEqual({
      kind: "unavailable",
      message: "Update status is unavailable.",
    });
    expect(
      settingsUpdateView({ isLoading: false, isError: true, error: new Error("offline") })
    ).toEqual({ kind: "error", message: "offline" });
  });

  it("Should preserve the last snapshot and identify a failed refresh", () => {
    expect(
      settingsUpdateView({
        data: snapshot,
        error: new Error("offline"),
        isError: true,
        isLoading: false,
      })
    ).toEqual({ kind: "snapshot", snapshot, refreshError: "offline" });
  });

  it("Should project a successful snapshot without an error", () => {
    expect(
      settingsUpdateView({ data: snapshot, error: null, isError: false, isLoading: false })
    ).toEqual({ kind: "snapshot", snapshot, refreshError: null });
  });
});

describe("settingsUpdateTracks", () => {
  it("Should derive both tracks with per-track versions, tones, and release links", () => {
    const tracks = settingsUpdateTracks(settingsUpdateBothAvailableFixture);

    expect(tracks.map(track => track.id)).toEqual(["runtime", "app"]);
    expect(tracks[0]).toMatchObject({
      label: "Runtime",
      status: "available",
      tone: "info",
      statusLabel: "Update available",
      currentVersion: "0.5.0",
      latestVersion: "0.5.1",
      releaseUrl: "https://github.com/compozy/compozy/releases/tag/v0.5.1",
      canApply: true,
      progress: null,
    });
    expect(tracks[1]).toMatchObject({
      label: "App",
      status: "available",
      currentVersion: "0.5.0",
      latestVersion: "0.5.1",
      canApply: true,
    });
  });

  it("Should omit the app track entirely when no desktop app is installed", () => {
    const tracks = settingsUpdateTracks(settingsUpdateNoAppFixture);

    expect(tracks).toHaveLength(1);
    expect(tracks[0].id).toBe("runtime");
  });

  it("Should refuse apply for a managed runtime and keep its recommendation verbatim", () => {
    const [runtime] = settingsUpdateTracks(settingsUpdateManagedFixture);

    expect(runtime.canApply).toBe(false);
    expect(runtime.recommendation).toBe("brew upgrade compozy");
    expect(runtime.tone).toBe("info");
  });

  it("Should refuse apply on every track while an operation holds the channel", () => {
    const tracks = settingsUpdateTracks(settingsUpdateStagedFixture);

    expect(tracks.every(track => track.canApply)).toBe(false);
  });

  it("Should attach live progress only to the operation's active target", () => {
    const [runtime] = settingsUpdateTracks(settingsUpdateApplyingFixture);

    expect(runtime.progress).toEqual({ phase: "install", percent: 62 });
  });

  it("Should report a staged app track with the daemon's next-launch message", () => {
    const app = settingsUpdateTracks(settingsUpdateStagedFixture).find(track => track.id === "app");

    expect(app).toMatchObject({
      status: "staged",
      tone: "info",
      statusLabel: "Staged",
      message: "CompozyOS app update staged; applies on next launch.",
    });
    // `active_target` is the app, but a dormant record is not mid-flight.
    expect(app?.progress).toBeNull();
    expect(app?.canCancel).toBe(true);
  });

  it("Should refuse to render another holder's progress against a blocked track", () => {
    const [runtime] = settingsUpdateTracks(settingsUpdateBlockedFixture);

    // The payload reports a live `downloading` operation, but it belongs to the
    // CLI holder — this track was refused, not started.
    expect(runtime.progress).toBeNull();
    expect(runtime.statusLabel).toBe("Blocked");
    expect(runtime.tone).toBe("warning");
    expect(runtime.canApply).toBe(false);
  });

  it("Should surface rollback truth as the restored version plus the last error", () => {
    const [runtime] = settingsUpdateTracks(settingsUpdateRolledBackFixture);

    expect(runtime).toMatchObject({
      status: "failed",
      tone: "danger",
      restoredVersion: "0.5.0",
      lastError: "health check failed after swap",
    });
    // The operation is terminal and the lease is free, so the offer stands again.
    expect(runtime.canApply).toBe(true);
  });

  it("Should describe a track only where the message adds truth beyond version and pill", () => {
    const [settled] = settingsUpdateTracks(settingsUpdateStatusFixture);
    const [offered] = settingsUpdateTracks(settingsUpdateBothAvailableFixture);
    const [managed] = settingsUpdateTracks(settingsUpdateManagedFixture);
    const [failed] = settingsUpdateTracks(settingsUpdateRolledBackFixture);

    expect(settled.description).toBeNull();
    expect(offered.description).toBeNull();
    expect(managed.description).toBe(
      "CompozyOS 0.5.1 is available. Upgrade with your package manager."
    );
    expect(failed.description).toBe("Update failed; restored CompozyOS runtime 0.5.0.");
  });
});

describe("settingsUpdateCancelable", () => {
  it("Should offer cancel only for a dormant operation nobody is executing", () => {
    expect(settingsUpdateCancelable(settingsUpdateStagedFixture)).toBe(true);
  });

  it("Should refuse cancel for a live executor lease and when no operation exists", () => {
    expect(settingsUpdateCancelable(settingsUpdateApplyingFixture)).toBe(false);
    expect(settingsUpdateCancelable(settingsUpdateBlockedFixture)).toBe(false);
    expect(settingsUpdateCancelable(settingsUpdateBothAvailableFixture)).toBe(false);
  });
});

describe("settingsUpdateIndicatorAvailable", () => {
  it("Should report available when either track offers an update and nothing is running", () => {
    expect(settingsUpdateIndicatorAvailable(settingsUpdateBothAvailableFixture)).toBe(true);
    expect(settingsUpdateIndicatorAvailable(settingsUpdateNoAppFixture)).toBe(true);
    expect(settingsUpdateIndicatorAvailable(settingsUpdateManagedFixture)).toBe(true);
  });

  it("Should suppress the indicator while an operation is live, whatever the tracks report", () => {
    expect(settingsUpdateIndicatorAvailable(settingsUpdateApplyingFixture)).toBe(false);
    expect(settingsUpdateIndicatorAvailable(settingsUpdateStagedFixture)).toBe(false);
    expect(settingsUpdateIndicatorAvailable(settingsUpdateBlockedFixture)).toBe(false);
  });

  it("Should stay hidden for up-to-date, failed, and absent snapshots", () => {
    expect(settingsUpdateIndicatorAvailable(settingsUpdateStatusFixture)).toBe(false);
    expect(settingsUpdateIndicatorAvailable(settingsUpdateRolledBackFixture)).toBe(false);
    expect(settingsUpdateIndicatorAvailable(undefined)).toBe(false);
  });
});

describe("updateUiPhase", () => {
  it("Should map every runtime journal phase onto the fixed UI vocabulary", () => {
    expect(updateUiPhase("runtime", "downloading", 10)).toBe("download");
    expect(updateUiPhase("runtime", "verifying", -1)).toBe("verify");
    expect(updateUiPhase("runtime", "swapping", -1)).toBe("install");
    expect(updateUiPhase("runtime", "restarting", -1)).toBe("start");
    expect(updateUiPhase("runtime", "health-checking", -1)).toBe("ready-check");
    expect(updateUiPhase("runtime", "finalized", -1)).toBe("ready");
  });

  it("Should map app phases, splitting `applying` on transfer completion", () => {
    expect(updateUiPhase("app", "applying", 50)).toBe("download");
    expect(updateUiPhase("app", "applying", 100)).toBe("verify");
    expect(updateUiPhase("app", "installer-handoff", -1)).toBe("install");
    expect(updateUiPhase("app", "restarted", -1)).toBe("start");
    expect(updateUiPhase("app", "verified", -1)).toBe("ready");
  });

  it("Should return null for phases with no measurable edge rather than inventing one", () => {
    expect(updateUiPhase("runtime", "pending", -1)).toBeNull();
    expect(updateUiPhase("runtime", "rolled-back", -1)).toBeNull();
    expect(updateUiPhase("app", "staged", -1)).toBeNull();
    expect(updateUiPhase("runtime", undefined, -1)).toBeNull();
    expect(updateUiPhase(undefined, "downloading", 10)).toBeNull();
    // A runtime-only phase must not leak into the app vocabulary.
    expect(updateUiPhase("app", "swapping", -1)).toBeNull();
  });
});

describe("updatePhasePercent", () => {
  it("Should keep measurable percents and drop the unmeasurable sentinel", () => {
    expect(updatePhasePercent(0)).toBe(0);
    expect(updatePhasePercent(62)).toBe(62);
    expect(updatePhasePercent(100)).toBe(100);
    expect(updatePhasePercent(-1)).toBeNull();
  });
});
