import { waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { toast } from "@compozy/ui";

import { SessionApiError } from "../../adapters/session-api";

import { createSessionPermissionDecisionLogic } from "../session-permission-decision-store";

beforeEach(() => {
  vi.spyOn(toast, "error").mockReturnValue("toast");
});

// Suite: session permission decision transitions
// Invariant: A result for an older permission request cannot resolve the currently displayed request.
// Boundary IN: Local submission phase and request fence transitions.
// Boundary OUT: Session approval transport and prompt rendering.
describe("session permission decision store", () => {
  it("Should ignore a real completion from a superseded permission request", async () => {
    const logic = createSessionPermissionDecisionLogic();
    const store = logic.createStore();
    let resolveFirst!: () => void;
    let resolveSecond!: () => void;
    const first = new Promise<void>(resolve => {
      resolveFirst = resolve;
    });
    const second = new Promise<void>(resolve => {
      resolveSecond = resolve;
    });
    const accepted = vi.fn();
    store.on("resolutionAccepted", accepted);

    store.trigger.decisionSubmitted({
      workspaceId: "ws_alpha",
      sessionId: "session-1",
      decision: "allow-once",
      permissionKey: "request-1",
      permission: {
        requestId: "request-1",
        turnId: "turn-1",
        toolName: "Bash",
        toolInput: {},
        action: "Run command",
        resource: "/workspace",
      },
      execute: () => first,
    });
    store.trigger.decisionSubmitted({
      workspaceId: "ws_alpha",
      sessionId: "session-1",
      decision: "reject-once",
      permissionKey: "request-2",
      permission: {
        requestId: "request-2",
        turnId: "turn-2",
        toolName: "Write",
        toolInput: {},
        action: "Write file",
        resource: "/workspace/file.txt",
      },
      execute: () => second,
    });

    resolveFirst();
    await waitFor(() => {
      expect(store.getSnapshot().context).toMatchObject({
        phase: "submitting",
        permissionKey: "request-2",
      });
    });
    expect(accepted).not.toHaveBeenCalled();

    resolveSecond();
    await waitFor(() => expect(store.getSnapshot().context.phase).toBe("resolved"));
    expect(accepted).toHaveBeenCalledOnce();
  });

  it.each([
    [
      409,
      "permission request expired",
      "This permission request expired. The action was not approved.",
    ],
    [
      409,
      "permission request canceled",
      "This permission request was canceled. The action was not approved.",
    ],
    [
      500,
      "internal database failure",
      "Failed to send permission response. The agent may continue waiting.",
    ],
  ])("Should present a safe permission failure for %s %s", async (status, cause, message) => {
    const store = createSessionPermissionDecisionLogic().createStore();
    const failure = new SessionApiError(`session: pending permission not found: ${cause}`, status);
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    try {
      store.trigger.decisionSubmitted({
        workspaceId: "ws_alpha",
        sessionId: "session-1",
        decision: "allow-once",
        permissionKey: "expired",
        permission: {
          requestId: "expired",
          turnId: "turn-1",
          toolName: "Session Stop",
          toolInput: {},
          action: "Stop session",
          resource: "session-2",
        },
        execute: vi.fn().mockRejectedValue(failure),
      });
      await waitFor(() => expect(toast.error).toHaveBeenCalledWith(message));
      expect(store.getSnapshot().context.phase).toBe("idle");
    } finally {
      consoleError.mockRestore();
    }
  });

  it("Should surface a rejected executor and return the active request to idle", async () => {
    const consoleError = vi.spyOn(console, "error").mockImplementation(() => undefined);
    try {
      const store = createSessionPermissionDecisionLogic().createStore();
      const failure = new Error("transport unavailable");
      vi.mocked(toast.error).mockClear();

      store.trigger.decisionSubmitted({
        workspaceId: "ws_alpha",
        sessionId: "session-1",
        decision: "reject-once",
        permissionKey: "request-1",
        permission: {
          requestId: "request-1",
          turnId: "turn-1",
          toolName: "Bash",
          toolInput: {},
          action: "Run command",
          resource: "/workspace",
        },
        execute: vi.fn().mockRejectedValue(failure),
      });

      await waitFor(() => expect(store.getSnapshot().context.phase).toBe("idle"));
      expect(toast.error).toHaveBeenCalledWith(
        "Failed to send permission response. The agent may continue waiting."
      );
      expect(consoleError).toHaveBeenCalledExactlyOnceWith(
        "Failed to submit permission decision",
        failure
      );
    } finally {
      consoleError.mockRestore();
    }
  });
});
