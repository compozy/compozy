// Suite: OS session-window content
// Invariant: a terminal process-exited session forks a child in the same workspace
// and preserves the original as the child's durable parent.
// Owning layer: the session-window recovery action.
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Suspense } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SessionPayload } from "@/systems/session";

const mocks = vi.hoisted(() => ({
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
    controls: {
      canPrompt: false,
      isBusyInputPending: false,
      isResuming: false,
      isSessionRunning: false,
      queuedPrompts: [],
      resumeFailure: null,
    },
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

vi.mock("@/systems/session", () => ({
  hasUnrecoverableRuntime: (session: SessionPayload) =>
    session.failure?.kind === "process_exit" && session.health?.health === "dead",
  SessionEnvironmentControl: () => null,
  SessionPromptRuntimeSelector: () => null,
  SessionResumeFailure: ({ onRetry, retryLabel }: { onRetry: () => void; retryLabel?: string }) => (
    <button type="button" onClick={onRetry}>
      {retryLabel}
    </button>
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

describe("SessionWindowContent", () => {
  beforeEach(() => {
    mocks.forkMutation.isPending = false;
    mocks.forkMutation.mutate.mockReset();
    mocks.selectSession.mockReset();
    mocks.sessionThreadProps.mockReset();
    mocks.toastError.mockReset();
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
