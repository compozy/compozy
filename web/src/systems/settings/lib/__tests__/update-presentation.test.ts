import { describe, expect, it } from "vitest";

import {
  settingsUpdateAppAvailableFixture,
  settingsUpdateApplyingFixture,
  settingsUpdateBlockedFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateManagedFixture,
  settingsUpdateNoAppFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateRuntimeAvailableFixture,
  settingsUpdateStagedFixture,
  settingsUpdateStatusFixture,
} from "../../mocks/settings-update-fixture";
import {
  settingsUpdateApplicableTargets,
  settingsUpdateTracks,
  settingsUpdateView,
} from "../update-presentation";
import { settingsProductUpdate, settingsUpdateIndicatorState } from "../update-product";
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

  it("Should preserve runtime-only and app-only offers as distinct track states", () => {
    const runtimeOnly = settingsUpdateTracks(settingsUpdateRuntimeAvailableFixture);
    const appOnly = settingsUpdateTracks(settingsUpdateAppAvailableFixture);

    expect(runtimeOnly.map(track => [track.id, track.canApply])).toEqual([
      ["runtime", true],
      ["app", false],
    ]);
    expect(appOnly.map(track => [track.id, track.canApply])).toEqual([
      ["runtime", false],
      ["app", true],
    ]);
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
    // Waiting clears `active_target`; the durable waiting state owns cancellation.
    expect(app?.progress).toBeNull();
    expect(app?.canCancel).toBe(true);
  });

  it("Should refuse cancel for a live holder and when no operation exists", () => {
    expect(
      settingsUpdateTracks(settingsUpdateApplyingFixture).every(track => !track.canCancel)
    ).toBe(true);
    expect(
      settingsUpdateTracks(settingsUpdateBlockedFixture).every(track => !track.canCancel)
    ).toBe(true);
    expect(
      settingsUpdateTracks(settingsUpdateBothAvailableFixture).every(track => !track.canCancel)
    ).toBe(true);
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
      tone: "destructive",
      restoredVersion: "0.5.0",
      lastError: "health check failed after swap",
    });
    // A failed projection is diagnostic-only until a fresh check reports available;
    // PlanOperation rejects failed targets even after the lease is free.
    expect(runtime.canApply).toBe(false);
  });

  it("Should not offer a failed track without release metadata", () => {
    const snapshotWithoutOffer = {
      ...settingsUpdateRolledBackFixture,
      runtime: {
        ...settingsUpdateRolledBackFixture.runtime,
        latest_version: "",
      },
    };

    const [runtime] = settingsUpdateTracks(snapshotWithoutOffer);

    expect(runtime.status).toBe("failed");
    expect(runtime.latestVersion).toBeNull();
    expect(runtime.canApply).toBe(false);
  });

  it("Should allow a failed app install to retry after its operation has settled", () => {
    const failed = {
      ...settingsUpdateAppAvailableFixture,
      aggregate: "failed" as const,
      app: {
        ...settingsUpdateAppAvailableFixture.app!,
        status: "failed" as const,
        last_error: "The installer did not finish.",
      },
    };

    expect(settingsProductUpdate(failed)).toMatchObject({
      status: "failed",
      targets: ["app"],
      updating: false,
      lastError: "The installer did not finish.",
    });
    expect(settingsUpdateIndicatorState(failed, false)).toBe("failed");
    expect(
      settingsProductUpdate({ ...failed, operation: settingsUpdateApplyingFixture.operation })
        .targets
    ).toBeNull();
    expect(
      settingsProductUpdate({ ...failed, app: { ...failed.app, latest_version: "" } }).targets
    ).toBeNull();
  });

  it("Should render unsupported as a neutral state with no mutation", () => {
    const unsupportedSnapshot = {
      ...settingsUpdateStatusFixture,
      runtime: {
        ...settingsUpdateStatusFixture.runtime,
        status: "unsupported" as const,
        message: "This install method cannot update itself.",
      },
    };

    const [runtime] = settingsUpdateTracks(unsupportedSnapshot);

    expect(runtime).toMatchObject({
      status: "unsupported",
      tone: "neutral",
      statusLabel: "Unsupported",
      canApply: false,
      canCancel: false,
    });
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

describe("settingsUpdateApplicableTargets", () => {
  it("Should preserve runtime-first order for both eligible tracks", () => {
    const tracks = settingsUpdateTracks(settingsUpdateBothAvailableFixture);

    expect(settingsUpdateApplicableTargets(tracks)).toEqual(["runtime", "app"]);
  });

  it("Should return exactly the eligible one-track target", () => {
    expect(
      settingsUpdateApplicableTargets(settingsUpdateTracks(settingsUpdateRuntimeAvailableFixture))
    ).toEqual(["runtime"]);
    expect(
      settingsUpdateApplicableTargets(settingsUpdateTracks(settingsUpdateAppAvailableFixture))
    ).toEqual(["app"]);
  });

  it("Should return no targets for managed, absent, or live update states", () => {
    expect(
      settingsUpdateApplicableTargets(settingsUpdateTracks(settingsUpdateManagedFixture))
    ).toBeNull();
    expect(
      settingsUpdateApplicableTargets(settingsUpdateTracks(settingsUpdateApplyingFixture))
    ).toBeNull();
    expect(
      settingsUpdateApplicableTargets(settingsUpdateTracks(settingsUpdateStatusFixture))
    ).toBeNull();
  });
});

describe("settingsProductUpdate", () => {
  it("Should fold both tracks into one offer with one version and one target set", () => {
    const product = settingsProductUpdate(settingsUpdateBothAvailableFixture);

    expect(product).toMatchObject({
      status: "available",
      statusLabel: "Update available",
      currentVersion: "0.5.0",
      latestVersion: "0.5.1",
      targets: ["runtime", "app"],
      updating: false,
      description: null,
      recommendation: null,
    });
  });

  it("Should offer the one eligible part without naming it", () => {
    expect(settingsProductUpdate(settingsUpdateAppAvailableFixture)).toMatchObject({
      status: "available",
      currentVersion: "0.5.1",
      latestVersion: null,
      targets: ["app"],
    });
    expect(settingsProductUpdate(settingsUpdateRuntimeAvailableFixture).targets).toEqual([
      "runtime",
    ]);
  });

  it("Should hand a managed install the upgrade command instead of a target set", () => {
    const product = settingsProductUpdate(settingsUpdateManagedFixture);

    expect(product.targets).toBeNull();
    expect(product.recommendation).toBe("brew upgrade compozy");
    expect(product.description).toBe("Update with your package manager to get the new version.");
  });

  it("Should keep the manual command when another part can update itself", () => {
    const product = settingsProductUpdate({
      ...settingsUpdateManagedFixture,
      app: settingsUpdateBothAvailableFixture.app,
    });

    expect(product.targets).toEqual(["app"]);
    expect(product.recommendation).toBe("brew upgrade compozy");
    expect(product.description).toBe("Update installs what it can here; do the rest by hand with:");
  });

  it("Should keep the manual command for an install this surface cannot update", () => {
    const product = settingsProductUpdate({
      ...settingsUpdateNoAppFixture,
      aggregate: "up-to-date",
      runtime: {
        ...settingsUpdateNoAppFixture.runtime,
        status: "unsupported",
        recommendation: "Download the latest release from GitHub.",
      },
    });

    expect(product.targets).toBeNull();
    expect(product.recommendation).toBe("Download the latest release from GitHub.");
    expect(product.description).toBe("Update this install manually with:");
  });

  it("Should not repeat a self-updating runtime's command beside the Update button", () => {
    const product = settingsProductUpdate({
      ...settingsUpdateBothAvailableFixture,
      runtime: {
        ...settingsUpdateBothAvailableFixture.runtime,
        recommendation: "Run `compozy update`.",
      },
    });

    expect(product.targets).toEqual(["runtime", "app"]);
    expect(product.recommendation).toBeNull();
    expect(product.description).toBeNull();
  });

  it("Should not claim nothing changed when a failure names no restored version", () => {
    const product = settingsProductUpdate({
      ...settingsUpdateRolledBackFixture,
      runtime: { ...settingsUpdateRolledBackFixture.runtime, restored_version: "" },
    });

    expect(product.description).toBe("The update didn't finish.");
  });

  it("Should report one live update with its measured percent", () => {
    expect(settingsProductUpdate(settingsUpdateApplyingFixture)).toMatchObject({
      updating: true,
      percent: 62,
      phase: "install",
      targets: null,
    });
  });

  it("Should treat another holder's operation as an update already running", () => {
    expect(settingsProductUpdate(settingsUpdateBlockedFixture)).toMatchObject({
      status: "blocked",
      updating: true,
      description: "An update is already running from the command line.",
    });
  });

  it("Should describe a staged update as finishing on next launch and allow cancel", () => {
    expect(settingsProductUpdate(settingsUpdateStagedFixture)).toMatchObject({
      status: "staged",
      updating: false,
      canCancel: true,
      description: "The update finishes the next time you open the CompozyOS app.",
    });
  });

  it("Should report a failed update with the restored version and its error", () => {
    expect(settingsProductUpdate(settingsUpdateRolledBackFixture)).toMatchObject({
      status: "failed",
      targets: null,
      description: "The update didn't finish, so CompozyOS went back to 0.5.0.",
      lastError: "health check failed after swap",
    });
  });
});

describe("settingsUpdateIndicatorState", () => {
  it("Should offer an update only while one is available and nothing runs", () => {
    expect(settingsUpdateIndicatorState(settingsUpdateBothAvailableFixture, false)).toBe(
      "available"
    );
    expect(settingsUpdateIndicatorState(settingsUpdateManagedFixture, false)).toBe("available");
    expect(settingsUpdateIndicatorState(settingsUpdateStatusFixture, false)).toBe("hidden");
    expect(settingsUpdateIndicatorState(settingsUpdateStagedFixture, false)).toBe("hidden");
    expect(settingsUpdateIndicatorState(undefined, false)).toBe("hidden");
  });

  it("Should show live work from the click until the operation settles", () => {
    // The request is in flight before the daemon journals anything.
    expect(settingsUpdateIndicatorState(settingsUpdateBothAvailableFixture, true)).toBe("updating");
    expect(settingsUpdateIndicatorState(settingsUpdateApplyingFixture, false)).toBe("updating");
    expect(settingsUpdateIndicatorState(settingsUpdateBlockedFixture, false)).toBe("updating");
  });

  it("Should surface a failed update instead of hiding it", () => {
    expect(settingsUpdateIndicatorState(settingsUpdateRolledBackFixture, false)).toBe("failed");
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
