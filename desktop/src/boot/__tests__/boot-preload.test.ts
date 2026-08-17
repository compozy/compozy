import { describe, expect, it } from "vitest";

import { validParams, validResponse, validState } from "../boot-contract";

// Invariant: every value crossing the boot preload bridge satisfies the complete public shape.
describe("boot preload contract", () => {
  it("Should reject malformed optional state fields", () => {
    expect(validState({ state: "starting", stage: "start", attempt: 2 })).toBe(true);
    expect(validState({ state: "starting", attempt: "2" })).toBe(false);
    expect(validState({ state: "product", origin: "https://example.com" })).toBe(false);
    expect(validState({ state: "product", origin: "http://localhost:2123", owned: false })).toBe(
      true
    );
    expect(validState({ state: "updating", target: "runtime", pct: 50 })).toBe(true);
    for (const value of [
      null,
      [],
      { state: "invented" },
      { state: "starting", stage: "invented" },
      { state: "starting", attempt: 6 },
      { state: "updating", pct: -1 },
      { state: "updating", target: "other" },
      { state: "skew", runtime: 1 },
      { state: "skew", needed: 1 },
      { state: "skew", newer: "yes" },
      { state: "error", error: { code: "", safe_message: "x", log_path: "x" } },
    ]) {
      expect(validState(value)).toBe(false);
    }
  });

  it("Should validate the complete diagnostic response", () => {
    const report = {
      schema_version: 1,
      boot_id: "boot-1",
      boot_phase: "error",
      app_version: "1.0.0",
      runtime_version: null,
      runtime_owned: null,
      current_error: { code: "runtime_failed", safe_message: "Runtime failed." },
      previous_crash: null,
    };
    expect(validResponse("diagnose", report)).toBe(true);
    expect(validResponse("diagnose", { ...report, runtime_owned: "yes" })).toBe(false);
    expect(validResponse("export_diagnostics", { bundle_path: "/tmp/x.tar.gz", bytes: -1 })).toBe(
      false
    );
    expect(validResponse("retry", { ok: true })).toBe(true);
    expect(validResponse("retry", { accepted: true })).toBe(true);
    expect(validResponse("copy_diagnostics", { copied: true })).toBe(true);
    expect(validResponse("open_logs", { opened: true })).toBe(true);
    expect(validResponse("quit", { quitting: true })).toBe(true);
    expect(validResponse("export_diagnostics", { bundle_path: "/tmp/x.tar.gz", bytes: 1 })).toBe(
      true
    );
    expect(validResponse("diagnose", { ...report, previous_crash: {} })).toBe(false);
    expect(validResponse("diagnose", { ...report, current_error: {} })).toBe(false);
  });

  it("Should accept only the parameters owned by each action", () => {
    expect(validParams("diagnose", {})).toBe(true);
    expect(validParams("diagnose", { extra: true })).toBe(false);
    expect(validParams("export_diagnostics", { consent: true })).toBe(true);
    expect(validParams("export_diagnostics", {})).toBe(false);
  });
});
