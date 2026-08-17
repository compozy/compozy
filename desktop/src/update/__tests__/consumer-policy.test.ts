import { describe, expect, it } from "vitest";

import { decideAppUpdateAction } from "../consumer-policy";
import type { AppOperationPhase, UpdateOperation } from "../operation-contract";

function operation(
  phase: AppOperationPhase,
  overrides: Partial<UpdateOperation> = {},
  active = true
): UpdateOperation {
  return {
    schema_version: 1,
    operation_id: "operation-1",
    revision: 4,
    targets: ["app"],
    ...(active ? { active_target: "app" as const } : {}),
    percent: -1,
    app: {
      from_version: "1.0.0",
      to_version: "1.1.0",
      release_tag: "v1.1.0",
      asset: "CompozyOS-1.1.0-arm64.zip",
      digest: "sha256:abc",
      attempt_id: "attempt-1",
      phase,
      consecutive_failures: 0,
    },
    holder: null,
    waiting: "",
    ...overrides,
  };
}

// Invariant: the shell journals ownership before install and only verifies a restart from observed version truth.
// Owner: desktop app-update policy. Canonical suite: consumer-policy.test.ts.
describe("app update consumer policy", () => {
  it("Should consume only a staged operation handed to the app", () => {
    const staged = operation("staged", { waiting: "waiting-for-app" }, false);
    expect(decideAppUpdateAction(staged, "1.0.0", new Date())).toEqual({ kind: "install" });
    expect(decideAppUpdateAction({ ...staged, waiting: "" }, "1.0.0", new Date())).toEqual({
      kind: "ignore",
    });
  });

  it("Should recover a staged operation after its shell lease expires", () => {
    const staged = operation("staged", {
      holder: {
        pid: 42,
        pid_start_time: "2026-08-16T09:00:00Z",
        surface: "shell",
        executor_generation: "old-shell",
        lease_expires_at: "2026-08-16T09:59:00Z",
      },
    });
    expect(decideAppUpdateAction(staged, "1.0.0", new Date("2026-08-16T10:00:00Z"))).toEqual({
      kind: "recover-install",
    });
    expect(
      decideAppUpdateAction(
        {
          ...staged,
          holder: { ...staged.holder!, lease_expires_at: "2026-08-16T10:01:00Z" },
        },
        "1.0.0",
        new Date("2026-08-16T10:00:00Z")
      )
    ).toEqual({ kind: "wait" });
  });

  it("Should gate verification on installed version truth", () => {
    const handoff = operation("installer-handoff");
    expect(decideAppUpdateAction(handoff, "1.1.0", new Date())).toEqual({
      kind: "verify-restart",
    });
    expect(decideAppUpdateAction(handoff, "1.0.0", new Date())).toEqual({ kind: "wait" });
    const tagged = {
      ...handoff,
      app: { ...handoff.app!, to_version: "v1.1.0" },
    };
    expect(decideAppUpdateAction(tagged, "1.1.0", new Date())).toEqual({
      kind: "verify-restart",
    });
  });

  it("Should fail a silent installer only after its durable watchdog deadline", () => {
    const handoff = operation("installer-handoff");
    const before = {
      ...handoff,
      app: { ...handoff.app!, watchdog_deadline: "2026-08-16T10:01:00Z" },
    };
    const expired = {
      ...handoff,
      app: { ...handoff.app!, watchdog_deadline: "2026-08-16T09:59:00Z" },
    };
    expect(decideAppUpdateAction(before, "1.0.0", new Date("2026-08-16T10:00:00Z"))).toEqual({
      kind: "wait",
    });
    expect(decideAppUpdateAction(expired, "1.0.0", new Date("2026-08-16T10:00:00Z"))).toEqual({
      kind: "fail-silent-install",
      reason: "The app installer did not replace version 1.0.0.",
    });
  });
});
