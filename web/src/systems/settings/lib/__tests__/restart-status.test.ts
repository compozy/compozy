import { describe, expect, it } from "vitest";

import {
  isFailedRestart,
  isSuccessfulRestart,
  isTerminalRestartStatus,
  RESTART_TERMINAL_STATUSES,
} from "../restart-status";
import { settingsRestartPresentation } from "../restart-presentation";

describe("restart status helpers", () => {
  it("treats only ready and failed as terminal", () => {
    expect(RESTART_TERMINAL_STATUSES).toEqual(["ready", "failed"]);
    expect(isTerminalRestartStatus("ready")).toBe(true);
    expect(isTerminalRestartStatus("failed")).toBe(true);
    expect(isTerminalRestartStatus("pending")).toBe(false);
    expect(isTerminalRestartStatus("stopping")).toBe(false);
    expect(isTerminalRestartStatus(null)).toBe(false);
    expect(isTerminalRestartStatus(undefined)).toBe(false);
  });

  it("distinguishes success from failure", () => {
    expect(isSuccessfulRestart("ready")).toBe(true);
    expect(isSuccessfulRestart("failed")).toBe(false);
    expect(isFailedRestart("failed")).toBe(true);
    expect(isFailedRestart("ready")).toBe(false);
  });
});

describe("settingsRestartPresentation", () => {
  const baseState = {
    isVisible: true,
    isRestartRequired: true,
    isPolling: false,
    isSuccessful: false,
    isFailed: false,
    operationId: null,
    status: null,
    activeSessionCount: 0,
    isTriggerPending: false,
    trigger: () => undefined,
    dismiss: () => undefined,
  };

  it.each([
    {
      name: "required",
      state: {},
      expected: {
        phase: "required",
        tone: "warning",
        title: "Restart needed",
        role: "status",
        dismissLabel: "Not now",
        triggerLabel: "Restart daemon",
        triggerPending: false,
      },
    },
    {
      name: "restart request pending",
      state: { isTriggerPending: true },
      expected: {
        phase: "required",
        tone: "warning",
        title: "Restart needed",
        role: "status",
        dismissLabel: null,
        triggerLabel: "Starting…",
        triggerPending: true,
      },
    },
    {
      name: "polling",
      state: { isPolling: true },
      expected: {
        phase: "polling",
        tone: "info",
        title: "Restarting…",
        role: "status",
        dismissLabel: null,
        triggerLabel: null,
        triggerPending: false,
      },
    },
    {
      name: "successful",
      state: { isRestartRequired: false, isSuccessful: true },
      expected: {
        phase: "successful",
        tone: "success",
        title: "Restarted",
        role: "status",
        dismissLabel: "Dismiss",
        triggerLabel: null,
        triggerPending: false,
      },
    },
    {
      name: "failed",
      state: { isFailed: true, failureReason: "helper spawn failed" },
      expected: {
        phase: "failed",
        tone: "danger",
        title: "Restart failed",
        role: "alert",
        dismissLabel: "Dismiss",
        triggerLabel: "Try again",
        triggerPending: false,
      },
    },
  ])("projects the $name phase exhaustively", ({ state, expected }) => {
    expect(settingsRestartPresentation({ ...baseState, ...state })).toMatchObject(expected);
  });

  it("omits unmeasured zero sessions and names measured active sessions", () => {
    expect(settingsRestartPresentation(baseState)?.sessionLabel).toBeNull();
    expect(settingsRestartPresentation({ ...baseState, activeSessionCount: 2 })?.sessionLabel).toBe(
      "2 active sessions right now."
    );
  });
});
