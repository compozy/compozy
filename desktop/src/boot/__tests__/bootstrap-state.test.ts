import { describe, expect, it } from "vitest";

import type { BootstrapEvent } from "../../bootstrap/bootstrap-event";
import { bootstrapSnapshot } from "../bootstrap-state";

function failed(message: string, phase: BootstrapEvent["phase"] = "ready"): BootstrapEvent {
  return { type: "bootstrap", phase, status: "failed", message };
}

// Invariant: typed bootstrap events map to the complete public shell state.
// Owner: desktop boot projection. Canonical suite: bootstrap-state.test.ts.
describe("bootstrapSnapshot", () => {
  it("Should keep runtime and app compatibility failures in the version-skew state", () => {
    expect(
      bootstrapSnapshot({
        ...failed("The runtime is incompatible."),
        compatibility: {
          reason: "runtime_below_minimum",
          runtime: "0.2.0",
          needed: ">=0.3.0-beta.8",
        },
      })
    ).toEqual({ state: "skew", runtime: "0.2.0", needed: ">=0.3.0-beta.8", newer: false });
    expect(
      bootstrapSnapshot({
        ...failed("The app is incompatible."),
        compatibility: {
          reason: "app_below_minimum",
          runtime: "0.4.0",
          needed: "0.4.0",
        },
      })
    ).toEqual({ state: "skew", runtime: "0.4.0", needed: "0.4.0", newer: true });
  });

  it.each(["start", "provision"] as const)(
    "Should classify a stalled data upgrade reported in the %s phase",
    phase => {
      expect(
        bootstrapSnapshot(
          failed(
            'cli: coordinated restart failed; runtime recovery copy retained at "/x": cli: daemon data upgrade "telemetry_relocation" stopped progressing at 1024 of 2048 rows for 5m0s',
            phase
          )
        )
      ).toMatchObject({ state: "error", error: { code: "runtime_upgrade_stalled" } });
    }
  );

  it("Should preserve the failed boot phase while redacting secrets", () => {
    expect(bootstrapSnapshot(failed("disk full\u0000 token=raw-secret", "provision"))).toEqual({
      state: "error",
      error: {
        code: "provision_disk_space",
        safe_message: "disk full  token=[redacted]",
        log_path: "desktop-bootstrap.jsonl",
      },
    });
  });

  it.each([
    [
      { type: "bootstrap", phase: "resolve", status: "started", message: "x" },
      { state: "resolving" },
    ],
    [
      { type: "bootstrap", phase: "attach", status: "started", message: "x" },
      { state: "attaching" },
    ],
    [
      { type: "bootstrap", phase: "provision", status: "started", message: "x" },
      { state: "provisioning", stage: "install" },
    ],
    [
      { type: "bootstrap", phase: "provision", status: "completed", message: "x" },
      { state: "provisioning", stage: "verify" },
    ],
    [
      {
        type: "bootstrap",
        phase: "provision",
        status: "started",
        message: "Upgrading CompozyOS data (42%).",
        stage: "migrate",
        pct: 42,
      },
      { state: "provisioning", stage: "migrate", pct: 42 },
    ],
    [
      {
        type: "bootstrap",
        phase: "provision",
        status: "started",
        message: "Upgrading CompozyOS data (0%).",
        stage: "migrate",
        pct: 0,
      },
      { state: "provisioning", stage: "migrate", pct: 0 },
    ],
    [
      { type: "bootstrap", phase: "start", status: "started", message: "x", attempt: 2 },
      { state: "starting", stage: "start", attempt: 2 },
    ],
    [
      { type: "bootstrap", phase: "start", status: "started", message: "x" },
      { state: "starting", stage: "start", attempt: 1 },
    ],
    [
      {
        type: "bootstrap",
        phase: "ready",
        status: "completed",
        message: "x",
        resolution: "attach",
      },
      { state: "attaching", stage: "ready" },
    ],
    [
      { type: "bootstrap", phase: "ready", status: "completed", message: "x", resolution: "start" },
      { state: "starting", stage: "ready", attempt: 1 },
    ],
  ] as const)("Should map bootstrap event %#", (event, expected) => {
    expect(bootstrapSnapshot(event)).toEqual(expected);
  });

  it("Should retain the actual start attempt until the product is ready", () => {
    const started = bootstrapSnapshot({
      type: "bootstrap",
      phase: "start",
      status: "started",
      message: "Retry",
      attempt: 3,
    });
    const ready = bootstrapSnapshot(
      {
        type: "bootstrap",
        phase: "ready",
        status: "completed",
        message: "Ready",
        resolution: "start",
      },
      "desktop-bootstrap.jsonl",
      started
    );
    expect(ready).toEqual({ state: "starting", stage: "ready", attempt: 3 });
    // The terminal failure event omits the attempt; the error keeps the one the start reached.
    expect(bootstrapSnapshot(failed("failed", "start"), "log", started)).toMatchObject({
      state: "error",
      attempt: 3,
    });
  });

  it("Should map determinate session upgrade progress to counts in sessions", () => {
    expect(
      bootstrapSnapshot({
        type: "bootstrap",
        phase: "provision",
        status: "started",
        message: "Upgrading CompozyOS data (29%).",
        stage: "migrate",
        pct: 29,
        completed: 744,
        total: 2554,
        migration: "session_history_upgrade",
      })
    ).toEqual({
      state: "provisioning",
      stage: "migrate",
      pct: 29,
      completed: 744,
      total: 2554,
      work: "sessions",
    });
  });

  it("Should keep the provisioned version transition until the boot ends", () => {
    const provisioning = bootstrapSnapshot({
      type: "bootstrap",
      phase: "provision",
      status: "started",
      message: "Provisioning",
      versions: { from: "0.5.4", to: "0.6.0" },
    });
    expect(provisioning).toEqual({
      state: "provisioning",
      stage: "install",
      from_version: "0.5.4",
      to_version: "0.6.0",
    });
    const migrating = bootstrapSnapshot(
      {
        type: "bootstrap",
        phase: "provision",
        status: "started",
        message: "Upgrading",
        stage: "migrate",
        pct: 0,
        completed: 0,
        total: 0,
        migration: "telemetry_relocation",
      },
      "desktop-bootstrap.jsonl",
      provisioning
    );
    expect(migrating).toMatchObject({
      work: "records",
      from_version: "0.5.4",
      to_version: "0.6.0",
    });
    const failure = bootstrapSnapshot(failed("failed", "start"), "log", migrating);
    expect(failure).toMatchObject({ state: "error", from_version: "0.5.4", to_version: "0.6.0" });
  });

  it.each([
    [
      { path: "/home/dora/.compozy/config.toml", unknown_keys: ["permissions.bogus"] },
      "Your config file has a setting CompozyOS does not recognize: permissions.bogus. Remove or rename it in /home/dora/.compozy/config.toml, then try again.",
    ],
    [
      { path: "/home/dora/.compozy/config.toml", unknown_keys: ["a", "b", "c", "d", "e", "f"] },
      "Your config file has settings CompozyOS does not recognize: a, b, c, d, e and 1 more. Remove or rename them in /home/dora/.compozy/config.toml, then try again.",
    ],
    [
      { path: "/home/dora/.compozy/config.toml", line: 3, column: 9 },
      "Your config file is not valid TOML at line 3, column 9. Fix /home/dora/.compozy/config.toml, then try again.",
    ],
    [{}, "cli: config could not be loaded"],
  ] as const)(
    "Should report config load failure %# with its cause and no start attempt",
    (config, safeMessage) => {
      const starting = bootstrapSnapshot({
        type: "bootstrap",
        phase: "start",
        status: "started",
        message: "Starting",
        attempt: 1,
      });
      expect(
        bootstrapSnapshot(
          {
            ...failed("cli: config could not be loaded", "failed"),
            config,
          },
          "desktop-bootstrap.jsonl",
          starting
        )
      ).toEqual({
        state: "error",
        error: {
          code: "config_invalid",
          safe_message: safeMessage,
          log_path: "desktop-bootstrap.jsonl",
        },
      });
    }
  );

  it.each([
    ["provision", "permission denied", "provision_permission"],
    ["provision", "network unavailable", "provision_network"],
    ["attach", "unhealthy", "runtime_unhealthy"],
    ["ready", "unhealthy", "runtime_unhealthy"],
    ["start", "failed", "runtime_start_failed"],
  ] as const)("Should classify %s failure", (phase, message, code) => {
    expect(bootstrapSnapshot(failed(message, phase))).toMatchObject({ error: { code } });
  });

  // Invariant: a wait on another process's runtime mutation is shown as a wait, never as a retry,
  // and giving up on it is a typed failure that names the cause instead of a start attempt.
  it("Should show a runtime mutation as a wait that keeps the start attempt", () => {
    expect(
      bootstrapSnapshot({
        type: "bootstrap",
        phase: "start",
        status: "started",
        message: "Another CompozyOS update or restart is in progress (pid 4242); waiting.",
        attempt: 1,
        classification: "mutation-in-progress",
        wait: { reason: "runtime_mutation", owner_pid: 4242 },
      })
    ).toEqual({
      state: "starting",
      stage: "start",
      attempt: 1,
      boot: { wait: "runtime_mutation", owner_pid: 4242 },
    });
  });

  it("Should fail a runtime mutation that outlasted the wait without counting an attempt", () => {
    const event: BootstrapEvent = {
      ...failed("cli: another CompozyOS update or restart did not finish", "failed"),
      classification: "mutation-in-progress",
    };
    const snapshot = bootstrapSnapshot(event, "desktop-bootstrap.jsonl", {
      state: "starting",
      stage: "start",
      attempt: 1,
      boot: { wait: "runtime_mutation" },
    });
    expect(snapshot).not.toHaveProperty("attempt");
    expect(snapshot.error?.code).toBe("update_lock_held");
    expect(snapshot.error?.safe_message).toBe(
      "Another CompozyOS update or restart is still running. Wait for it to finish, then try again."
    );
  });

  it("Should fail a runtime that cannot be launched with the bootstrap's reason", () => {
    const message = "cli: the CompozyOS runtime cannot be launched: permission denied";
    const event: BootstrapEvent = {
      ...failed(message, "failed"),
      classification: "runtime-unlaunchable",
    };
    const snapshot = bootstrapSnapshot(event, "desktop-bootstrap.jsonl", {
      state: "starting",
      stage: "start",
      attempt: 1,
    });
    expect(snapshot).toEqual({
      state: "error",
      attempt: 1,
      error: {
        code: "runtime_start_failed",
        safe_message: message,
        log_path: "desktop-bootstrap.jsonl",
      },
    });
  });

  it("Should carry the runtime's lifecycle phase and step while it starts", () => {
    expect(
      bootstrapSnapshot({
        type: "bootstrap",
        phase: "start",
        status: "started",
        message: "The CompozyOS runtime is starting.",
        attempt: 1,
        readiness: { phase: "serving", step: "resumeBootInterruptedSessions" },
      })
    ).toEqual({
      state: "starting",
      stage: "start",
      attempt: 1,
      boot: { readiness: "serving", step: "resumeBootInterruptedSessions" },
    });
  });
});
