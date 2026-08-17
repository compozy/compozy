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
      { type: "bootstrap", phase: "start", status: "started", message: "x", attempt: 2 },
      { state: "starting", stage: "start", attempt: 2 },
    ],
    [
      { type: "bootstrap", phase: "start", status: "started", message: "x" },
      { state: "starting", stage: "start" },
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
      { state: "starting", stage: "ready" },
    ],
  ] as const)("Should map bootstrap event %#", (event, expected) => {
    expect(bootstrapSnapshot(event)).toEqual(expected);
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
