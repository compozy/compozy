import { describe, expect, it } from "vitest";

import type { BootstrapEvent } from "../../bootstrap/bootstrap-event";
import { bootstrapSnapshot } from "../bootstrap-state";

function failed(message: string, phase: BootstrapEvent["phase"] = "ready"): BootstrapEvent {
  return { type: "bootstrap", phase, status: "failed", message };
}

describe("bootstrapSnapshot", () => {
  it("keeps runtime and app compatibility failures in the version-skew state", () => {
    expect(
      bootstrapSnapshot(
        failed("cli: runtime 0.2.0 does not satisfy >=0.3.0-beta.8; repair the runtime")
      )
    ).toEqual({ state: "skew", runtime: "0.2.0", needed: ">=0.3.0-beta.8", newer: false });
    expect(
      bootstrapSnapshot(
        failed(
          "cli: update: runtime 0.4.0 requires CompozyOS app 0.4.0 or newer; installed app is 0.3.0"
        )
      )
    ).toEqual({ state: "skew", runtime: "0.4.0", needed: "0.4.0", newer: true });
  });

  it("preserves the failed boot phase and a public-safe cause", () => {
    expect(bootstrapSnapshot(failed("disk full\u0000", "provision"))).toEqual({
      state: "error",
      error: {
        code: "provision_disk_space",
        safe_message: "disk full",
        log_path: "desktop-bootstrap.jsonl",
      },
    });
  });
});
