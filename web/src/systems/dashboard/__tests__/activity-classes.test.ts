import { describe, expect, it } from "vitest";

import {
  homeActivityTone,
  isQuietActivityEvent,
  isTaskLifecycleEvent,
} from "../lib/activity-classes";

describe("isQuietActivityEvent", () => {
  it("Should fold tool calls and config reads behind the quiet disclosure", () => {
    expect(isQuietActivityEvent({ type: "tool_call", outcome: "info" })).toBe(true);
    expect(isQuietActivityEvent({ type: "tool.call_completed", outcome: "success" })).toBe(true);
    expect(isQuietActivityEvent({ type: "config.read", outcome: "info" })).toBe(true);
    expect(isQuietActivityEvent({ type: "memory.compaction_completed", outcome: "info" })).toBe(
      true
    );
  });

  it("Should keep lifecycle and problem events loud", () => {
    expect(isQuietActivityEvent({ type: "task.run_completed", outcome: "success" })).toBe(false);
    expect(isQuietActivityEvent({ type: "message", outcome: "info" })).toBe(false);
    expect(isQuietActivityEvent({ type: "tool.call_failed", outcome: "failure" })).toBe(false);
    expect(isQuietActivityEvent({ type: "hook.dispatch.complete", outcome: "warning" })).toBe(
      false
    );
  });
});

describe("homeActivityTone", () => {
  it("Should remap failure to danger and default missing outcomes to neutral", () => {
    expect(homeActivityTone({ outcome: "failure" })).toBe("danger");
    expect(homeActivityTone({ outcome: "info" })).toBe("neutral");
    expect(homeActivityTone({})).toBe("neutral");
  });
});

describe("isTaskLifecycleEvent", () => {
  it("Should match every task-family event", () => {
    expect(isTaskLifecycleEvent({ type: "task.approved" })).toBe(true);
    expect(isTaskLifecycleEvent({ type: "task.run_failed" })).toBe(true);
    expect(isTaskLifecycleEvent({ type: "task.custom_future_event" })).toBe(true);
    expect(isTaskLifecycleEvent({ type: "session.started" })).toBe(false);
  });
});
