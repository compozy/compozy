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
    ["provision", "permission denied", "provision_permission"],
    ["provision", "network unavailable", "provision_network"],
    ["attach", "unhealthy", "runtime_unhealthy"],
    ["ready", "unhealthy", "runtime_unhealthy"],
    ["start", "failed", "runtime_start_failed"],
  ] as const)("Should classify %s failure", (phase, message, code) => {
    expect(bootstrapSnapshot(failed(message, phase))).toMatchObject({ error: { code } });
  });
});
