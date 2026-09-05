// Suite: OS session-window content
// Invariant: recovery preserves durable lineage, composer cancellation never
// stops the session, and the window's notice slot surfaces the daemon's
// runtime and stop truth with the one action that exists for each.
// Owning layer: the session-window recovery and stop-attention actions.
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Suspense } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "@/systems/session";

const mocks = vi.hoisted(() => ({
  cancelPrompt: vi.fn(),
  controls: {
    canPrompt: false,
    canRetryStop: true,
    handleCancelPrompt: vi.fn(),
    handleStop: vi.fn(),
    isBusyInputPending: false,
    isResuming: false,
    isSessionRunning: false,
    isStopRetrying: false,
    queuedPrompts: [],
    resumeFailure: null,
    stopAttention: null as string | null,
  },
  forkMutation: { isPending: false, mutate: vi.fn() },
  selectSession: vi.fn(),
  sessionThreadProps: vi.fn(),
  toastError: vi.fn(),
}));

vi.mock("../session-window-module-loader", () => ({
  loadSessionThread: async () => ({
    SessionThread: (props: Record<string, unknown>) => {
      mocks.sessionThreadProps(props);
      return null;
    },
  }),
}));

vi.mock("../use-session-window-controller", () => ({
  useSessionWindowController: () => ({
    clearDialog: { open: false },
    commandCatalog: [],
    commandCatalogStatus: "ready",
    controls: mocks.controls,
    deleteDialog: { open: false },
    inspector: { open: false },
    inspectorMemory: {},
    inspectorUsage: null,
    refreshCommandCatalog: vi.fn(),
    promptRuntimeSnapshot: null,
    renameDialog: { open: false },
    sessionVault: { data: [], error: null, isLoading: false },
    sidebar: {
      collapsedThreadIds: [],
      disconnected: false,
      onNewSession: vi.fn(),
      onSelectSession: mocks.selectSession,
      onToggleThread: vi.fn(),
      open: false,
      rowDeleteDialog: { open: false },
      rowRenameDialog: { open: false },
      sessionActions: {},
      sessions: [],
      toggle: vi.fn(),
    },
  }),
}));

vi.mock("@/systems/session", async () => ({
  // The stop-attention notice is under test here: the real composite, not a stand-in.
  ...(await import("@/systems/session/components/session-stop-attention-notice")),
  hasUnrecoverableRuntime: (session: SessionPayload) =>
    session.failure?.kind === "process_exit" && session.health?.health === "dead",
  SessionEnvironmentControl: () => null,
  SessionPromptRuntimeSelector: () => null,
  SessionResumeFailure: ({ onRetry, retryLabel }: { onRetry: () => void; retryLabel?: string }) => (
    <button type="button" onClick={onRetry}>
      {retryLabel}
    </button>
  ),
  SessionRuntimeRecoveryNotice: ({
    attempt,
    maxAttempts,
  }: {
    attempt?: number;
    maxAttempts?: number;
  }) => (
    <div role="status">
      Recovering runtime · Attempt {attempt} of {maxAttempts}
    </div>
  ),
  SessionSidebar: () => null,
  useCreateSession: () => mocks.forkMutation,
}));

vi.mock("sonner", () => ({ toast: { error: mocks.toastError } }));

import { SessionWindowContent } from "../session-window-content";

const deadSession: SessionPayload = {
  profile_id: "00000000000000000000000000",
  profile_name: "default",
  agent_name: "codex-agent",
  archived_at: null,
  attachable: false,
  available_commands: [],
  pending_interactions: [],
  badge: "stopped",
  created_at: "2026-08-13T12:00:00Z",
  failure: { kind: "process_exit", summary: "Codex exited" },
  health: {
    active_prompt: false,
    agent_name: "codex-agent",
    attachable: false,
    eligible_for_wake: false,
    health: "dead",
    session_id: "sess-dead",
    state: "stopped",
    updated_at: "2026-08-13T12:01:00Z",
    workspace_id: "ws-alpha",
  },
  id: "sess-dead",
  runtime: { selection_revision: 0, status: "unbound" },
  state: "stopped",
  updated_at: "2026-08-13T12:01:00Z",
  workspace_id: "ws-alpha",
};

const unverifiedStopSession = {
  ...deadSession,
  attention: "stop_verification_failed",
  badge: "needs-attention",
  escalated: true,
  failure: undefined,
  health: undefined,
  id: "sess-unverified",
  runtime: { effective: { provider: "codex" }, selection_revision: 0, status: "ready" },
  state: "stopping",
} satisfies SessionPayload;

function renderUnverifiedStop() {
  return render(
    <Suspense fallback={null}>
      <SessionWindowContent
        agentName="codex-agent"
        liveDataEnabled={false}
        onDeleteSuccess={vi.fn()}
        session={unverifiedStopSession}
        sessionId={unverifiedStopSession.id}
        windowId={`session:${unverifiedStopSession.id}`}
        workspaceId="ws-alpha"
      />
    </Suspense>
  );
}

describe("SessionWindowContent", () => {
  beforeEach(() => {
    mocks.cancelPrompt.mockReset();
    mocks.controls.canRetryStop = true;
    mocks.controls.handleCancelPrompt = mocks.cancelPrompt;
    mocks.controls.handleStop.mockReset();
    mocks.controls.isStopRetrying = false;
    mocks.controls.stopAttention = null;
    mocks.forkMutation.isPending = false;
    mocks.forkMutation.mutate.mockReset();
    mocks.selectSession.mockReset();
    mocks.sessionThreadProps.mockReset();
    mocks.toastError.mockReset();
  });

  it("Should bind Stop generation to prompt cancellation instead of session stopping", async () => {
    const session = {
      ...deadSession,
      failure: undefined,
      health: undefined,
      runtime: {
        effective: { provider: "codex" },
        selection_revision: 0,
        status: "ready",
      },
      state: "active",
    } satisfies SessionPayload;

    render(
      <Suspense fallback={null}>
        <SessionWindowContent
          agentName="codex-agent"
          liveDataEnabled={false}
          onDeleteSuccess={vi.fn()}
          session={session}
          sessionId={session.id}
          windowId={`session:${session.id}`}
          workspaceId="ws-alpha"
        />
      </Suspense>
    );

    await waitFor(() => expect(mocks.sessionThreadProps).toHaveBeenCalled());
    const props = mocks.sessionThreadProps.mock.lastCall?.[0] as {
      onCancelPrompt: () => void;
    };
    act(() => props.onCancelPrompt());

    expect(mocks.cancelPrompt).toHaveBeenCalledOnce();
    expect(mocks.controls.handleStop).not.toHaveBeenCalled();
  });

  // Invariant (US-009.AC-3, ADR-004 invariant 3): a stop the daemon could not
  // verify surfaces in the notice slot as a warning that stays while the
  // session reads stopping; Retry is the session stop action itself.
  it("Should surface an unverified stop and retry it through the session stop action", async () => {
    mocks.controls.stopAttention = "stop_verification_failed";

    renderUnverifiedStop();

    const notice = await screen.findByTestId("session-stop-attention");
    expect(notice).toHaveAttribute("role", "alert");
    expect(notice).toHaveAttribute("data-attention", "stop_verification_failed");
    expect(screen.getByTestId("session-stop-attention-title")).toHaveTextContent(
      "Couldn’t confirm the agent stopped."
    );
    expect(screen.getByTestId("session-stop-attention-meta")).toHaveTextContent(
      "stop_verification_failed"
    );
    expect(screen.queryByRole("button", { name: "Fork into a new session" })).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Retry stop" }));

    expect(mocks.controls.handleStop).toHaveBeenCalledOnce();
    expect(mocks.cancelPrompt).not.toHaveBeenCalled();
    // The thread still reads the stop in progress: the composer pill holds.
    await waitFor(() =>
      expect(mocks.sessionThreadProps).toHaveBeenLastCalledWith(
        expect.objectContaining({ sessionState: "stopping" })
      )
    );
  });

  it("Should hold Retry while a retry is landing and omit it for a managed session", async () => {
    mocks.controls.stopAttention = "stop_verification_failed";
    mocks.controls.isStopRetrying = true;

    const { rerender } = renderUnverifiedStop();

    // The button keeps its name while waiting: the spinner is decorative, the state is aria-busy.
    const retry = await screen.findByRole("button", { name: "Retry stop" });
    expect(retry).toBeDisabled();
    expect(retry).toHaveAttribute("aria-busy", "true");
    fireEvent.click(retry);
    expect(mocks.controls.handleStop).not.toHaveBeenCalled();

    mocks.controls.isStopRetrying = false;
    mocks.controls.canRetryStop = false;
    rerender(
      <Suspense fallback={null}>
        <SessionWindowContent
          agentName="codex-agent"
          liveDataEnabled={false}
          onDeleteSuccess={vi.fn()}
          session={{ ...unverifiedStopSession, type: "system" }}
          sessionId={unverifiedStopSession.id}
          windowId={`session:${unverifiedStopSession.id}`}
          workspaceId="ws-alpha"
        />
      </Suspense>
    );

    expect(screen.getByTestId("session-stop-attention")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry stop" })).toBeNull();
  });

  it("Should fork a dead session into its workspace and select the child", async () => {
    render(
      <Suspense fallback={null}>
        <SessionWindowContent
          agentName="codex-agent"
          liveDataEnabled={false}
          onDeleteSuccess={vi.fn()}
          session={deadSession}
          sessionId="sess-dead"
          windowId="session:sess-dead"
          workspaceId="ws-alpha"
        />
      </Suspense>
    );

    fireEvent.click(await screen.findByRole("button", { name: "Fork into a new session" }));

    expect(mocks.forkMutation.mutate).toHaveBeenCalledWith(
      {
        agent_name: "codex-agent",
        parent_session_id: "sess-dead",
        workspace: "ws-alpha",
      },
      expect.objectContaining({ onError: expect.any(Function), onSuccess: expect.any(Function) })
    );
    const callbacks = mocks.forkMutation.mutate.mock.calls[0]?.[1];
    const child = { ...deadSession, failure: undefined, health: undefined, id: "sess-child" };
    callbacks?.onSuccess(child);

    expect(mocks.selectSession).toHaveBeenCalledWith(child);
  });

  it("Should show automatic recovery progress while the runtime is recovering", async () => {
    const recoveringSession = {
      ...deadSession,
      failure: undefined,
      health: undefined,
      runtime: {
        generation: 3,
        recovery: {
          attempt: 2,
          generation: 3,
          last_attempt_at: "2026-08-22T16:00:02Z",
          max_attempts: 3,
          started_at: "2026-08-22T16:00:00Z",
        },
        selection_revision: 0,
        status: "recovering",
      },
      state: "active",
    } satisfies SessionPayload;

    render(
      <Suspense fallback={null}>
        <SessionWindowContent
          agentName="codex-agent"
          liveDataEnabled={false}
          onDeleteSuccess={vi.fn()}
          session={recoveringSession}
          sessionId={recoveringSession.id}
          windowId={`session:${recoveringSession.id}`}
          workspaceId="ws-alpha"
        />
      </Suspense>
    );

    expect(await screen.findByRole("status")).toHaveTextContent(
      "Recovering runtime · Attempt 2 of 3"
    );
    expect(
      screen.queryByRole("button", { name: "Fork into a new session" })
    ).not.toBeInTheDocument();
  });

  it("Should treat absent ACP capabilities as unknown", async () => {
    const session = {
      ...deadSession,
      failure: undefined,
      health: undefined,
      runtime: {
        effective: { model: "active-model", provider: "codex" },
        selected: { model: "active-model", provider: "codex" },
        selection_revision: 1,
        status: "ready",
      },
      state: "active",
    } satisfies SessionPayload;

    render(
      <Suspense fallback={null}>
        <SessionWindowContent
          agentName="codex-agent"
          liveDataEnabled={false}
          onDeleteSuccess={vi.fn()}
          session={session}
          sessionId={session.id}
          windowId={`session:${session.id}`}
          workspaceId="ws-alpha"
        />
      </Suspense>
    );

    await waitFor(() => {
      expect(mocks.sessionThreadProps).toHaveBeenLastCalledWith(
        expect.objectContaining({
          promptEmbeddedContextCapability: "unknown",
          promptImageCapability: "unknown",
        })
      );
    });
  });

  it("Should treat stale ACP capabilities as unknown", async () => {
    const session = {
      ...deadSession,
      failure: undefined,
      health: undefined,
      runtime: {
        acp_caps: {
          prompt_audio: false,
          prompt_embedded_context: false,
          prompt_image: false,
          supports_load_session: false,
        },
        effective: { model: "previous-model", provider: "codex" },
        selected: { model: "next-model", provider: "codex" },
        selection_revision: 1,
        status: "ready",
      },
      state: "active",
    } satisfies SessionPayload;

    render(
      <Suspense fallback={null}>
        <SessionWindowContent
          agentName="codex-agent"
          liveDataEnabled={false}
          onDeleteSuccess={vi.fn()}
          session={session}
          sessionId={session.id}
          windowId={`session:${session.id}`}
          workspaceId="ws-alpha"
        />
      </Suspense>
    );

    await waitFor(() => {
      expect(mocks.sessionThreadProps).toHaveBeenLastCalledWith(
        expect.objectContaining({
          promptEmbeddedContextCapability: "unknown",
          promptImageCapability: "unknown",
        })
      );
    });
  });
});
