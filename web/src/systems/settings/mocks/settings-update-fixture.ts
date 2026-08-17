import type { SettingsUpdateStatus } from "@/systems/settings";

/**
 * Update-surface fixtures, one per state `_uiux.md` S1/S2 enumerate. Every shape
 * is the daemon's live MultiState projection: an always-present runtime track, an
 * app track only when a desktop app is installed, and a non-null operation exactly
 * while one is live on this home.
 */

const RELEASE_URL = "https://github.com/compozy/compozy/releases/tag/v0.5.1";

/** Both tracks current, nothing running — the default at rest. */
export const settingsUpdateStatusFixture: SettingsUpdateStatus = {
  aggregate: "up-to-date",
  operation: null,
  runtime: {
    status: "up-to-date",
    install_method: "direct-binary",
    managed: false,
    current_version: "0.5.1",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "",
    daemon_restarted: false,
    message: "CompozyOS runtime is up to date.",
    last_error: "",
  },
  app: {
    status: "up-to-date",
    running: true,
    current_version: "0.5.1",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    attempt_id: "",
    message: "CompozyOS app is up to date.",
    last_error: "",
  },
};

/** Both tracks offer 0.5.1 — the two-track available state. */
export const settingsUpdateBothAvailableFixture: SettingsUpdateStatus = {
  aggregate: "available",
  operation: null,
  runtime: {
    status: "available",
    install_method: "desktop-app",
    managed: false,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "",
    daemon_restarted: false,
    message: "CompozyOS runtime 0.5.1 is available.",
    last_error: "",
  },
  app: {
    status: "available",
    running: true,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    attempt_id: "",
    message: "CompozyOS app 0.5.1 is available.",
    last_error: "",
  },
};

/** Runtime offers an update; the installed app is already current. */
export const settingsUpdateRuntimeAvailableFixture: SettingsUpdateStatus = {
  ...settingsUpdateBothAvailableFixture,
  app: {
    status: "up-to-date",
    running: true,
    current_version: "0.5.1",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    attempt_id: "",
    message: "CompozyOS app is up to date.",
    last_error: "",
  },
};

/** The installed app offers an update; the runtime is already current. */
export const settingsUpdateAppAvailableFixture: SettingsUpdateStatus = {
  ...settingsUpdateBothAvailableFixture,
  runtime: {
    ...settingsUpdateStatusFixture.runtime,
    install_method: "desktop-app",
  },
};

/** Package-manager install — recommendation only, never a mutation. */
export const settingsUpdateManagedFixture: SettingsUpdateStatus = {
  aggregate: "available",
  operation: null,
  runtime: {
    status: "available",
    install_method: "homebrew",
    managed: true,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "brew upgrade compozy",
    restored_version: "",
    daemon_restarted: false,
    message: "CompozyOS 0.5.1 is available. Upgrade with your package manager.",
    last_error: "",
  },
  app: null,
};

/** Headless host — no desktop app installed, so the app track is absent. */
export const settingsUpdateNoAppFixture: SettingsUpdateStatus = {
  aggregate: "available",
  operation: null,
  runtime: {
    status: "available",
    install_method: "direct-binary",
    managed: false,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "",
    daemon_restarted: false,
    message: "CompozyOS runtime 0.5.1 is available.",
    last_error: "",
  },
  app: null,
};

/** Runtime apply in flight, mid-swap — the live progress state. */
export const settingsUpdateApplyingFixture: SettingsUpdateStatus = {
  aggregate: "applying",
  operation: {
    id: "op-7f3a2c",
    revision: 4,
    targets: ["runtime"],
    active_target: "runtime",
    phase: "swapping",
    percent: 62,
    waiting: "",
    started_at: "2026-08-20T14:02:11Z",
    last_error: "",
    holder: {
      pid: 4242,
      pid_start_time: "2026-08-20T14:02:10Z",
      surface: "daemon",
      executor_generation: "gen-1",
      lease_expires_at: "2026-08-20T14:07:11Z",
    },
  },
  runtime: {
    status: "applying",
    install_method: "desktop-app",
    managed: false,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "",
    daemon_restarted: false,
    message: "Updating CompozyOS runtime to 0.5.1.",
    last_error: "",
  },
  app: null,
};

/** App closed — its update is staged and applies on next launch (dormant record). */
export const settingsUpdateStagedFixture: SettingsUpdateStatus = {
  aggregate: "staged",
  operation: {
    id: "op-7f3a2c",
    revision: 9,
    targets: ["runtime", "app"],
    phase: "staged",
    percent: -1,
    waiting: "waiting-for-app",
    started_at: "2026-08-20T14:02:11Z",
    last_error: "",
    holder: null,
  },
  runtime: {
    status: "updated",
    install_method: "desktop-app",
    managed: false,
    current_version: "0.5.1",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "",
    daemon_restarted: true,
    message: "Updated CompozyOS runtime to 0.5.1 and restarted the daemon.",
    last_error: "",
  },
  app: {
    status: "staged",
    running: false,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    attempt_id: "attempt-1",
    message: "CompozyOS app update staged; applies on next launch.",
    last_error: "",
  },
};

/** Another surface holds the update channel — single-flight refusal. */
export const settingsUpdateBlockedFixture: SettingsUpdateStatus = {
  aggregate: "blocked",
  operation: {
    id: "op-7f3a2c",
    revision: 2,
    targets: ["runtime"],
    active_target: "runtime",
    phase: "downloading",
    percent: 12,
    waiting: "",
    started_at: "2026-08-20T14:02:11Z",
    last_error: "",
    holder: {
      pid: 4242,
      pid_start_time: "2026-08-20T14:02:10Z",
      surface: "cli",
      executor_generation: "gen-1",
      lease_expires_at: "2026-08-20T14:07:11Z",
    },
  },
  runtime: {
    status: "blocked",
    install_method: "desktop-app",
    managed: false,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "",
    daemon_restarted: false,
    message: "A runtime update is already in progress (holder pid 4242). Retry after it completes.",
    last_error: "",
  },
  app: null,
};

/** Apply failed and the previous binary was restored. */
export const settingsUpdateRolledBackFixture: SettingsUpdateStatus = {
  aggregate: "failed",
  operation: null,
  runtime: {
    status: "failed",
    install_method: "desktop-app",
    managed: false,
    current_version: "0.5.0",
    latest_version: "0.5.1",
    release_url: RELEASE_URL,
    recommendation: "",
    restored_version: "0.5.0",
    daemon_restarted: true,
    message: "Update failed; restored CompozyOS runtime 0.5.0.",
    last_error: "health check failed after swap",
  },
  app: null,
};
