// Suite: OS session-window controller
// Invariant: inspector-only projections fetch only while the inspector is open in a live window.
// Boundary IN: retained-window liveness and inspector query admission.
// Boundary OUT: query transport behavior, owned by each domain hook suite.
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "@/systems/session";
import { primarySessionFixture } from "@/systems/session/mocks";

const mocks = vi.hoisted(() => ({
  inspectorOpen: false,
  sessionLedger: vi.fn(),
  sessionUsage: vi.fn(),
  sessionVault: vi.fn(),
}));

vi.mock("@/hooks/routes/use-session-clear-dialog", () => ({
  useSessionClearDialog: () => ({ openDialog: vi.fn() }),
}));

vi.mock("@/hooks/routes/use-session-delete-dialog", () => ({
  useSessionDeleteDialog: () => ({ openDialog: vi.fn() }),
}));

vi.mock("@/hooks/routes/use-session-page-controls", () => ({
  useSessionPageControls: () => ({
    canClear: true,
    handleClear: vi.fn(),
    handleDelete: vi.fn(),
    handleDismissResumeFailure: vi.fn(),
    handleInterruptPrompt: vi.fn(),
    handleQueuePrompt: vi.fn(),
    handleRemoveQueuedPrompt: vi.fn(),
    handleReplaceQueuedPrompt: vi.fn(),
    handleResume: vi.fn(),
    handleSteerPrompt: vi.fn(),
    handleSteerQueuedPrompt: vi.fn(),
    handleStop: vi.fn(),
    isClearing: false,
    isDeleting: false,
    isResuming: false,
    isSessionRunning: false,
    isStopping: false,
    isUnarchiving: false,
  }),
}));

vi.mock("../use-session-window-sidebar", () => ({
  useSessionWindowSidebar: () => ({ open: false, toggle: vi.fn() }),
}));

vi.mock("@/systems/session", () => ({
  getSessionPromptRuntimeSnapshot: vi.fn(),
  SessionGoalHeadAction: () => null,
  useSessionCommands: () => ({ catalog: [], isPending: false, refetch: vi.fn() }),
  useSessionGoalHeader: () => ({
    onApprove: vi.fn(),
    onClear: vi.fn(),
    onPause: vi.fn(),
    onResume: vi.fn(),
    pendingAction: null,
    snapshot: null,
  }),
  useSessionInspectorState: () => ({ open: mocks.inspectorOpen, toggle: vi.fn() }),
  useSessionLedger: (...args: unknown[]) => {
    mocks.sessionLedger(...args);
    return { data: undefined, error: null, isLoading: false };
  },
  useSessionPromptRuntimeContext: () => null,
  useSessionTopbarSlot: vi.fn(),
  useSessionUsage: (...args: unknown[]) => {
    mocks.sessionUsage(...args);
    return { data: undefined };
  },
}));

vi.mock("@/systems/vault", () => ({
  useSessionVaultSecrets: (...args: unknown[]) => {
    mocks.sessionVault(...args);
    return { data: undefined, error: null, isLoading: false };
  },
}));

import { useSessionWindowController } from "../use-session-window-controller";

const session: SessionPayload = {
  ...primarySessionFixture,
  id: "sess-1",
  state: "stopped",
  workspace_id: "ws-1",
};

describe("useSessionWindowController", () => {
  beforeEach(() => {
    mocks.inspectorOpen = false;
    mocks.sessionLedger.mockReset();
    mocks.sessionUsage.mockReset();
    mocks.sessionVault.mockReset();
  });

  it("Should defer inspector-only reads until the inspector opens in a live window", () => {
    const input = {
      windowId: "window:sess-1",
      sessionId: "sess-1",
      workspaceId: "ws-1",
      session,
      onDeleteSuccess: vi.fn(),
      liveDataEnabled: true,
    };
    const { rerender } = renderHook(() => useSessionWindowController(input));

    expect(mocks.sessionVault).toHaveBeenLastCalledWith("sess-1", { enabled: false });
    expect(mocks.sessionLedger).toHaveBeenLastCalledWith("sess-1", "ws-1", { enabled: false });
    expect(mocks.sessionUsage).toHaveBeenLastCalledWith("sess-1", "ws-1", "stopped", {
      enabled: false,
    });

    mocks.inspectorOpen = true;
    rerender();

    expect(mocks.sessionVault).toHaveBeenLastCalledWith("sess-1", { enabled: true });
    expect(mocks.sessionLedger).toHaveBeenLastCalledWith("sess-1", "ws-1", { enabled: true });
    expect(mocks.sessionUsage).toHaveBeenLastCalledWith("sess-1", "ws-1", "stopped", {
      enabled: true,
    });
  });
});
