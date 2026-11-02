import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import {
  buildTerminalQuote,
  holdPendingTerminalQuote,
  peekPendingTerminalQuote,
  takePendingTerminalQuote,
} from "@/systems/terminal/parts";
import type { SessionPayload } from "../../types";
import { SessionCreateProvider } from "../../contexts/session-create-context";
import {
  clearSessionTerminalQuote,
  peekSessionTerminalQuote,
} from "../../lib/session-terminal-quote";
import { createSessionCreateStore } from "../../stores/session-create-store";
import { sessionStore } from "../../stores/session-store";
import { useSessionCreateActions } from "../use-session-create";
import {
  useSessionCreateDialogController,
  useSessionCreateDialogViewModel,
  type SessionCreateDialogApi,
} from "../use-session-create-dialog";
import type { AgentPayload } from "@/systems/agent";
import { OsShellContext, type OsShellHandle } from "@/systems/os";
import type { DataScope, WorkspacePayload } from "@/systems/workspace";
import { LAST_SESSION_PROJECT_STORAGE_KEY } from "../../lib/session-create-target";
import {
  buildWorktreeFixture,
  emptyWorktreeListingFixture,
} from "@/systems/workspace/mocks/worktree-fixtures";

// Invariant: creation submits only durable session identity and launch context, binds creation
// outside a project to the home path without a worktree choice, materializes a project environment
// before binding to it, and hands off to the canonical session route without inventing a prompt.
// A prompt already sent from the composer is staged separately while its agent fallback waits.
// Every exit from a pending environment rolls the worktree back without losing that staged prompt.
// Owning layer: session-create view model. Canonical suite: this hook test.
const {
  mockCancel,
  mockMaterializationRef,
  mockMutateAsync,
  mockNavigate,
  mockReset,
  mockSetActiveWorkspaceId,
  mockStart,
  mockUseAgents,
  mockUserHomeDir,
  mockWorkspaceListRef,
  mockWorktreeListingRef,
  mockActive,
  mockProfileScope,
  mockProfiles,
} = vi.hoisted(() => ({
  mockActive: {
    current: {
      breadthMode: "focused" as "focused" | "all",
      pending: false,
      projectWorkspaces: [] as unknown[],
      dataScope: { kind: "project", workspaceId: "ws_alpha" } as unknown,
    },
  },
  mockProfileScope: { current: { aggregate: false, destination: "default" } },
  mockProfiles: {
    current: [] as { name: string; archived_at: string | null }[],
  },
  mockCancel: vi.fn(),
  mockMaterializationRef: { current: { status: "idle" as string, worktree: undefined as unknown } },
  mockMutateAsync: vi.fn<(input: unknown) => Promise<SessionPayload>>(),
  mockNavigate: vi.fn<(input: unknown) => Promise<void>>(),
  mockReset: vi.fn(),
  mockSetActiveWorkspaceId: vi.fn<(workspaceId: string | null) => void>(),
  mockStart: vi.fn(),
  mockUseAgents: vi.fn(),
  mockUserHomeDir: { current: undefined as string | undefined },
  mockWorkspaceListRef: { current: [] as WorkspacePayload[] },
  mockWorktreeListingRef: { current: undefined as unknown },
}));

vi.mock("@/systems/workspace/hooks/use-worktrees", () => ({
  useWorktrees: () => ({ data: mockWorktreeListingRef.current }),
}));

vi.mock("@/systems/workspace/hooks/use-worktree-materialization", () => ({
  useWorktreeMaterialization: () => ({
    ...mockMaterializationRef.current,
    cancel: mockCancel,
    recover: mockReset,
    reset: mockReset,
    retry: vi.fn(),
    start: mockStart,
  }),
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mockNavigate }));

vi.mock("@/systems/agent/hooks/use-agents", () => ({
  useAgents: (workspaceId: string, options?: { enabled?: boolean; profile?: string }) =>
    mockUseAgents(workspaceId, options),
}));

vi.mock("@/systems/workspace/stores/active-workspace-store", () => ({
  setActiveWorkspaceId: mockSetActiveWorkspaceId,
}));

vi.mock("@/systems/workspace/hooks/use-user-home-dir", () => ({
  useUserHomeDir: () => mockUserHomeDir.current,
}));

vi.mock("@/systems/workspace/hooks/use-workspaces", () => ({
  useWorkspaces: () => ({ data: mockWorkspaceListRef.current, error: null, isLoading: false }),
}));

vi.mock("../use-session-actions", () => ({
  useCreateSession: () => ({ mutateAsync: mockMutateAsync, isPending: false }),
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    activeWorkspaceId: "ws_alpha",
    runtimeWorkspaceId: "ws_alpha",
    runtimeWorkspace: { default_agent: "codex-agent" },
    ...mockActive.current,
  }),
}));

vi.mock("@/systems/profiles/hooks/use-profile-read-scope", async importOriginal => ({
  ...(await importOriginal<object>()),
  useProfileReadScope: () => mockProfileScope.current,
}));

vi.mock("@/systems/profiles/hooks/use-profiles", async importOriginal => ({
  ...(await importOriginal<object>()),
  useProfiles: () => ({ data: mockProfiles.current }),
}));

vi.mock("@/systems/workspace/hooks/use-active-worktree", async importOriginal => ({
  ...(await importOriginal<object>()),
  useScopedWorktreeFilter: () => ({
    worktreeId: undefined,
    resolved: true,
  }),
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

const activeWorkspace: WorkspacePayload = {
  id: "ws_alpha",
  root_dir: "/workspace/alpha",
  add_dirs: [],
  name: "alpha",
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:00Z",
};

const homeWorkspace: WorkspacePayload = {
  ...activeWorkspace,
  id: "ws_home",
  root_dir: "/Users/operator",
  name: "operator-home",
};

const agents: AgentPayload[] = [
  {
    label: "Claude Agent",
    kind: "on_demand",
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "claude-agent",
      generation: 1,
      color: "#7c5cff",
    },
    name: "claude-agent",
    provider: "claude",
    prompt: "help",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    label: "Codex Agent",
    kind: "on_demand",
    resolved_appearance: {
      version: 1,
      avatar: "generated",
      seed: "codex-agent",
      generation: 1,
      color: "#7c5cff",
    },
    name: "codex-agent",
    provider: "codex",
    prompt: "code",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

const createdSession: SessionPayload = {
  board_column: "done",
  pull_request: null,
  pull_request_lookup: null,
  supervision: null,
  profile_id: "00000000000000000000000000",
  profile_name: "default",
  scope: "workspace",
  effective_workspace_access: "",
  id: "sess-new",
  agent_name: "codex-agent",
  workspace_id: "ws_alpha",
  workspace_path: "/workspace/alpha",
  state: "active",
  badge: "idle",
  attachable: true,
  archived_at: null,
  available_commands: [],
  pending_interactions: [],
  runtime: { status: "unbound", selection_revision: 0 },
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:01Z",
};

function createWrapper(shell: OsShellHandle | null = null) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(
      OsShellContext,
      { value: shell },
      createElement(QueryClientProvider, { client: queryClient }, children)
    );
  return { wrapper };
}

interface TestSessionCreateDialogApi extends SessionCreateDialogApi {
  openDialog: (agentName: string, target?: { project?: string; profile?: string }) => void;
  pendingPrompt: string | null;
  stageFallbackPrompt: (prompt: string) => void;
}

function useSessionCreateDialog(context: {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
  dataScope?: DataScope | null;
  projectWorkspaceId?: string | null;
  focusedBinding?: string | null;
}): TestSessionCreateDialogApi {
  const controller = useSessionCreateDialogController();
  const dialog = useSessionCreateDialogViewModel(context, controller.store);
  const pendingPrompt = useSelector(controller.store, snapshot => snapshot.context.pendingPrompt);
  return {
    ...dialog,
    pendingPrompt,
    openDialog: (agentName, target) => {
      controller.store.trigger.dialogOpened({
        agentName,
        workspaceId: context.activeWorkspace?.id ?? "",
        ...(target ? { target } : {}),
      });
    },
    stageFallbackPrompt: prompt => controller.store.trigger.fallbackPromptStaged({ prompt }),
  };
}

function renderCreateQuoteLifecycle() {
  const store = createSessionCreateStore();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <SessionCreateProvider store={store}>{children}</SessionCreateProvider>
    </QueryClientProvider>
  );
  const rendered = renderHook(
    () => ({
      actions: useSessionCreateActions(),
      dialog: useSessionCreateDialogViewModel({ agents, activeWorkspace }, store),
    }),
    { wrapper }
  );
  return { ...rendered, store };
}

function useSessionCreateControllerPair() {
  const first = useSessionCreateDialogController();
  const second = useSessionCreateDialogController();
  return {
    firstDraftAgentName: useSelector(first.store, snapshot => snapshot.context.draft.agentName),
    firstStore: first.store,
    secondDraftAgentName: useSelector(second.store, snapshot => snapshot.context.draft.agentName),
  };
}

describe("useSessionCreateDialog", () => {
  beforeEach(() => {
    mockMutateAsync.mockReset();
    mockMutateAsync.mockResolvedValue(createdSession);
    mockNavigate.mockReset();
    mockNavigate.mockResolvedValue(undefined);
    mockSetActiveWorkspaceId.mockReset();
    mockUseAgents.mockReset();
    mockUseAgents.mockReturnValue({ data: undefined });
    mockUserHomeDir.current = undefined;
    mockWorkspaceListRef.current = [activeWorkspace];
    mockCancel.mockReset();
    mockReset.mockReset();
    mockStart.mockReset();
    mockMaterializationRef.current = { status: "idle", worktree: undefined };
    // The real store flips to `creating` synchronously inside `start`, which is
    // what keeps the armed submit waiting instead of standing down at once.
    mockStart.mockImplementation(() => {
      mockMaterializationRef.current = { status: "creating", worktree: undefined };
    });
    // Most cases are not git-backed, so the environment control is absent and
    // creation behaves exactly as it did before worktrees existed.
    mockWorktreeListingRef.current = undefined;
    mockActive.current = {
      breadthMode: "focused",
      pending: false,
      projectWorkspaces: [activeWorkspace],
      dataScope: { kind: "project", workspaceId: "ws_alpha" },
    };
    mockProfileScope.current = { aggregate: false, destination: "default" };
    mockProfiles.current = [];
    window.localStorage.removeItem(LAST_SESSION_PROJECT_STORAGE_KEY);
    sessionStore.trigger.sessionInteractionRemoved({ sessionId: createdSession.id });
    takePendingTerminalQuote();
    clearSessionTerminalQuote(createdSession.id);
  });

  it("Should isolate draft state between dialog controller instances", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateControllerPair(), { wrapper });

    act(() => {
      result.current.firstStore.trigger.dialogOpened({
        agentName: "claude-agent",
        workspaceId: activeWorkspace.id,
      });
    });

    expect(result.current.firstDraftAgentName).toBe("claude-agent");
    expect(result.current.secondDraftAgentName).toBe("general");
  });

  it("Should expose the active workspace and select the requested agent", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));

    expect(result.current.workspaceId).toBe("ws_alpha");
    expect(result.current.selectedAgentName).toBe("codex-agent");
  });

  it("Should preselect general for an unspecified launch but block when it is unavailable", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog(""));

    expect(result.current.selectedAgentName).toBe("general");
    await act(async () => result.current.submit());

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe("Select an agent before starting the session.");
  });

  it("Should submit a durable session without prompt or runtime overrides", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));
    act(() => result.current.onSessionNameChange("  Investigate checkout latency  "));
    await act(async () => result.current.submit());

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      name: "Investigate checkout latency",
      workspace: "ws_alpha",
    });
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name/sessions/$id",
        params: { name: "codex-agent", id: "sess-new" },
      })
    );
    expect(mockSetActiveWorkspaceId).not.toHaveBeenCalled();
    expect(result.current.restoreFocusOnClose).toBe(false);
    expect(sessionStore.getSnapshot().context.firstPrompts[createdSession.id]).toBeUndefined();
  });

  it("Should restore focus after dismissal but not after handing off to the created session", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));
    expect(result.current.restoreFocusOnClose).toBe(true);

    act(() => result.current.onOpenChange(false));
    expect(result.current.restoreFocusOnClose).toBe(true);

    act(() => result.current.openDialog("codex-agent"));
    await act(async () => result.current.submit());
    await waitFor(() => expect(result.current.restoreFocusOnClose).toBe(false));
  });

  it("Should name the created session to the shell before it switches workspace and navigates", async () => {
    const otherWorkspace: WorkspacePayload = { ...activeWorkspace, id: "ws_beta", name: "beta" };
    mockWorkspaceListRef.current = [activeWorkspace, otherWorkspace];
    mockMutateAsync.mockResolvedValue({
      ...createdSession,
      id: "sess-beta",
      workspace_id: otherWorkspace.id,
    });
    const noteCreatedSession = vi.fn();
    const shell = { coordinator: { noteCreatedSession } } as unknown as OsShellHandle;

    const { wrapper } = createWrapper(shell);
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));
    await act(async () => result.current.submit());

    await waitFor(() => expect(mockNavigate).toHaveBeenCalled());
    // The coordinator lands the new session in an existing Session window (#825).
    expect(noteCreatedSession).toHaveBeenCalledExactlyOnceWith("sess-beta");
    expect(mockSetActiveWorkspaceId).toHaveBeenCalledWith(otherWorkspace.id);
    expect(noteCreatedSession.mock.invocationCallOrder[0]).toBeLessThan(
      mockSetActiveWorkspaceId.mock.invocationCallOrder[0] ?? Number.POSITIVE_INFINITY
    );
  });

  it("Should activate the created session workspace before navigating", async () => {
    const otherWorkspace: WorkspacePayload = { ...activeWorkspace, id: "ws_beta", name: "beta" };
    const sessionInOtherWorkspace: SessionPayload = {
      ...createdSession,
      agent_name: "claude-agent",
      id: "sess-beta",
      workspace_id: otherWorkspace.id,
      workspace_path: otherWorkspace.root_dir,
    };
    mockWorkspaceListRef.current = [activeWorkspace, otherWorkspace];
    mockMutateAsync.mockResolvedValue(sessionInOtherWorkspace);

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("claude-agent"));
    await act(async () => result.current.submit());

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name/sessions/$id",
        params: { name: "claude-agent", id: sessionInOtherWorkspace.id },
      });
    });
    expect(mockSetActiveWorkspaceId).toHaveBeenCalledWith(otherWorkspace.id);
    expect(mockSetActiveWorkspaceId.mock.invocationCallOrder[0]).toBeLessThan(
      mockNavigate.mock.invocationCallOrder[0] ?? Number.POSITIVE_INFINITY
    );
  });

  it("W-07 Should start a User session in User without a project picker, worktree or activation", async () => {
    mockUserHomeDir.current = "/Users/operator";
    mockWorktreeListingRef.current = emptyWorktreeListingFixture;
    mockMutateAsync.mockResolvedValue({
      ...createdSession,
      workspace_id: "",
      workspace_path: "/Users/operator",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useSessionCreateDialog({
          agents,
          activeWorkspace: homeWorkspace,
          dataScope: { kind: "user" },
          projectWorkspaceId: activeWorkspace.id,
        }),
      { wrapper }
    );

    act(() => result.current.openDialog("codex-agent"));
    expect(result.current.environment).toBeUndefined();
    expect(result.current.destinationAxes.project).toBeUndefined();
    await act(async () => result.current.submit());

    expect(mockMutateAsync).toHaveBeenCalledWith({ agent_name: "codex-agent", scope: "user" });
    await waitFor(() => expect(mockNavigate).toHaveBeenCalled());
    expect(mockSetActiveWorkspaceId).not.toHaveBeenCalled();
  });

  it("Should report validation when the selected agent is empty", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("claude-agent"));
    act(() => result.current.onAgentChange(""));
    await act(async () => result.current.submit());

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe("Select an agent before starting the session.");
  });

  it("Should queue only a prompt staged by the composer fallback", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));
    act(() => result.current.stageFallbackPrompt("Investigate the regression"));
    await act(async () => result.current.submit());

    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledTimes(1));
    expect(sessionStore.getSnapshot().context.firstPrompts[createdSession.id]).toEqual({
      text: "Investigate the regression",
      claimed: false,
    });
  });

  it("Should stage a held quote onto the session create just produced", async () => {
    const quote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    holdPendingTerminalQuote(quote);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));
    await act(async () => result.current.submit());

    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledTimes(1));
    expect(peekSessionTerminalQuote(createdSession.id)?.text).toBe(quote.text);
    expect(peekPendingTerminalQuote()).toBeNull();
  });

  it("Should stage only the quote captured at submit when Start mutates during create", async () => {
    const firstQuote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    const laterQuote = buildTerminalQuote({
      terminalId: "term-9cd7e14b2a66",
      fromLine: 40,
      lines: ["second start"],
    });
    holdPendingTerminalQuote(firstQuote);
    let resolveCreate: ((session: SessionPayload) => void) | undefined;
    mockMutateAsync.mockImplementation(
      () =>
        new Promise(resolve => {
          resolveCreate = resolve;
        })
    );
    const { result, store } = renderCreateQuoteLifecycle();

    act(() => {
      store.trigger.dialogOpened({
        agentName: "codex-agent",
        workspaceId: activeWorkspace.id,
      });
    });
    act(() => result.current.dialog.submit());

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledTimes(1);
      expect(store.getSnapshot().context.operation.status).toBe("submitting");
    });
    expect(peekPendingTerminalQuote()).toBeNull();

    act(() => {
      result.current.actions.openForAgent("claude-agent");
      result.current.actions.openWithTerminalQuote(laterQuote);
    });

    expect(store.getSnapshot().context.draft.agentName).toBe("codex-agent");
    expect(peekPendingTerminalQuote()).toBeNull();

    await act(async () => {
      resolveCreate?.(createdSession);
    });

    await waitFor(() => {
      expect(peekSessionTerminalQuote(createdSession.id)?.text).toBe(firstQuote.text);
    });
    expect(peekPendingTerminalQuote()).toBeNull();
    expect(peekSessionTerminalQuote(createdSession.id)?.text).not.toBe(laterQuote.text);
  });

  it("Should restore the captured quote after create fails when nothing newer is pending", async () => {
    const firstQuote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    const laterQuote = buildTerminalQuote({
      terminalId: "term-9cd7e14b2a66",
      fromLine: 40,
      lines: ["second start"],
    });
    holdPendingTerminalQuote(firstQuote);
    let rejectCreate: ((error: Error) => void) | undefined;
    mockMutateAsync.mockImplementation(
      () =>
        new Promise((_resolve, reject) => {
          rejectCreate = reject;
        })
    );
    const { result, store } = renderCreateQuoteLifecycle();

    act(() => {
      store.trigger.dialogOpened({
        agentName: "codex-agent",
        workspaceId: activeWorkspace.id,
      });
    });
    act(() => result.current.dialog.submit());

    await waitFor(() => expect(store.getSnapshot().context.operation.status).toBe("submitting"));

    act(() => {
      result.current.actions.openForAgent("claude-agent");
      result.current.actions.openWithTerminalQuote(laterQuote);
    });

    await act(async () => {
      rejectCreate?.(new Error("agent executable is unavailable"));
    });

    await waitFor(() => expect(store.getSnapshot().context.operation.status).toBe("idle"));
    expect(peekPendingTerminalQuote()?.text).toBe(firstQuote.text);
    expect(peekSessionTerminalQuote(createdSession.id)).toBeNull();
  });

  it("Should drop a held quote when the create dialog is dismissed", () => {
    const quote = buildTerminalQuote({
      terminalId: "term-4f21c9a03b7e",
      fromLine: 12,
      lines: ["FAIL src/api/users.test.ts"],
    });
    holdPendingTerminalQuote(quote);
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openDialog("codex-agent"));
    act(() => result.current.onOpenChange(false));

    expect(peekPendingTerminalQuote()).toBeNull();
    expect(peekSessionTerminalQuote(createdSession.id)).toBeNull();
  });

  describe("with a new worktree as the environment", () => {
    const readyWorktree = buildWorktreeFixture({
      id: "wt_hotfix",
      name: "hotfix-cors",
      state: "ready",
    });

    function armSubmit() {
      mockWorktreeListingRef.current = emptyWorktreeListingFixture;
      const { wrapper } = createWrapper();
      const rendered = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => rendered.result.current.openDialog("codex-agent"));
      act(() => rendered.result.current.stageFallbackPrompt("Investigate the regression"));
      // Through the environment field's handler — the seam the dialog actually renders,
      // so a regression that materializes on selection is visible here.
      act(() =>
        rendered.result.current.environment?.onChange({
          kind: "new",
          name: "",
          previous: { kind: "root" },
        })
      );
      return rendered;
    }

    it("Should create the worktree on submit rather than on selection", async () => {
      const { result, rerender } = armSubmit();

      // Choosing is not a commitment: an abandoned dialog leaves nothing behind.
      expect(mockStart).not.toHaveBeenCalled();

      act(() => result.current.submit());

      expect(mockStart).toHaveBeenCalledTimes(1);
      expect(mockMutateAsync).not.toHaveBeenCalled();
      expect(result.current.isAwaitingEnvironment).toBe(true);

      mockMaterializationRef.current = { status: "ready", worktree: readyWorktree };
      await act(async () => rerender());

      await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledTimes(1));
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ worktree: "wt_hotfix" })
      );
      await waitFor(() =>
        expect(sessionStore.getSnapshot().context.firstPrompts[createdSession.id]?.text).toBe(
          "Investigate the regression"
        )
      );
    });

    it("Should cancel the worktree on Cancel and keep the dialog and staged prompt", async () => {
      const { result, rerender } = armSubmit();
      act(() => result.current.submit());
      await act(async () => rerender());

      act(() => result.current.onCancelEnvironment());

      expect(mockCancel).toHaveBeenCalledTimes(1);
      expect(result.current.open).toBe(true);
      expect(result.current.pendingPrompt).toBe("Investigate the regression");
      expect(result.current.environment?.value).toEqual({ kind: "root" });
      expect(result.current.isAwaitingEnvironment).toBe(false);
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it("Should restore the prior ready worktree when a new materialization is canceled", async () => {
      mockWorktreeListingRef.current = {
        ...emptyWorktreeListingFixture,
        worktrees: [readyWorktree],
      };
      const { wrapper } = createWrapper();
      const rendered = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => rendered.result.current.openDialog("codex-agent"));
      act(() =>
        rendered.result.current.environment?.onChange({
          kind: "worktree",
          worktreeId: readyWorktree.id,
        })
      );
      act(() =>
        rendered.result.current.environment?.onChange({
          kind: "new",
          name: "",
          previous: { kind: "worktree", worktreeId: readyWorktree.id },
        })
      );
      act(() => rendered.result.current.submit());
      act(() => rendered.result.current.onCancelEnvironment());

      expect(rendered.result.current.environment?.value).toEqual({
        kind: "worktree",
        worktreeId: readyWorktree.id,
      });
    });

    it("Should cancel a pending worktree when the dialog is dismissed", async () => {
      const { result, rerender } = armSubmit();
      act(() => result.current.submit());
      await act(async () => rerender());

      act(() => result.current.onOpenChange(false));

      expect(mockCancel).toHaveBeenCalledTimes(1);
      expect(result.current.open).toBe(false);
    });

    it("Should stage the quote captured when the worktree submit was armed", async () => {
      const quote = buildTerminalQuote({
        terminalId: "term-4f21c9a03b7e",
        fromLine: 12,
        lines: ["FAIL src/api/users.test.ts"],
      });
      holdPendingTerminalQuote(quote);
      const { result, rerender } = armSubmit();

      act(() => result.current.submit());

      expect(peekPendingTerminalQuote()).toBeNull();
      expect(result.current.isAwaitingEnvironment).toBe(true);

      mockMaterializationRef.current = { status: "ready", worktree: readyWorktree };
      await act(async () => rerender());

      await waitFor(() =>
        expect(peekSessionTerminalQuote(createdSession.id)?.text).toBe(quote.text)
      );
      expect(peekPendingTerminalQuote()).toBeNull();
    });

    it("Should restore the captured quote when a worktree wait is canceled", () => {
      const quote = buildTerminalQuote({
        terminalId: "term-4f21c9a03b7e",
        fromLine: 12,
        lines: ["FAIL src/api/users.test.ts"],
      });
      holdPendingTerminalQuote(quote);
      const { result } = armSubmit();

      act(() => result.current.submit());
      expect(peekPendingTerminalQuote()).toBeNull();

      act(() => result.current.onCancelEnvironment());

      expect(peekPendingTerminalQuote()?.text).toBe(quote.text);
      expect(peekSessionTerminalQuote(createdSession.id)).toBeNull();
    });

    it("Should stand down when materialization fails, leaving the draft and its exits intact", async () => {
      const { result, rerender } = armSubmit();
      act(() => result.current.submit());
      await act(async () => rerender());

      mockMaterializationRef.current = { status: "failed", worktree: undefined };
      await act(async () => rerender());

      expect(result.current.isAwaitingEnvironment).toBe(false);
      expect(mockMutateAsync).not.toHaveBeenCalled();
      expect(result.current.pendingPrompt).toBe("Investigate the regression");
      // Retry, pick another environment, and fall back to the root are all still
      // on the table — the field renders them from this same model.
      expect(result.current.environment?.materialization.status).toBe("failed");
      expect(result.current.environment?.onChange).toBeTypeOf("function");
    });
  });

  describe("destination pickers under the aggregates (ADR-003)", () => {
    const betaWorkspace: WorkspacePayload = {
      ...activeWorkspace,
      id: "ws_beta",
      root_dir: "/workspace/beta",
      name: "beta",
    };

    function allProjects(projectWorkspaces: WorkspacePayload[]) {
      mockActive.current = {
        breadthMode: "all",
        pending: false,
        projectWorkspaces,
        dataScope: { kind: "all" },
      };
      mockUseAgents.mockReturnValue({ data: agents });
    }

    it("W-13 Should prefill the focused window's project and land in place", async () => {
      allProjects([activeWorkspace, betaWorkspace]);
      mockMutateAsync.mockResolvedValue({ ...createdSession, workspace_id: "ws_beta" });
      const { wrapper } = createWrapper();
      const { result } = renderHook(
        () =>
          useSessionCreateDialog({
            agents,
            activeWorkspace: undefined,
            dataScope: { kind: "all" },
            focusedBinding: "ws_beta",
          }),
        { wrapper }
      );

      act(() => result.current.openDialog("codex-agent"));
      expect(result.current.destinationAxes.project?.value).toBe("ws_beta");
      expect(result.current.destinationAxes.project?.options.map(option => option.value)).toEqual([
        "ws_alpha",
        "ws_beta",
        "@user",
      ]);
      await act(async () => result.current.submit());

      expect(mockMutateAsync).toHaveBeenCalledWith({
        agent_name: "codex-agent",
        workspace: "ws_beta",
      });
      await waitFor(() =>
        expect(mockNavigate).toHaveBeenCalledWith({
          to: "/agents/$name/sessions/$id",
          params: { name: "codex-agent", id: "sess-new" },
          search: { workspace: "ws_beta" },
        })
      );
      // All projects stays on: the created session never switches the destination.
      expect(mockSetActiveWorkspaceId).not.toHaveBeenCalled();
      expect(window.localStorage.getItem(LAST_SESSION_PROJECT_STORAGE_KEY)).toBe("ws_beta");
    });

    it("W-14 Should fall back to the first project when the last-used one is gone", async () => {
      allProjects([activeWorkspace, betaWorkspace]);
      window.localStorage.setItem(LAST_SESSION_PROJECT_STORAGE_KEY, "ws_c");
      const { wrapper } = createWrapper();
      const { result } = renderHook(
        () =>
          useSessionCreateDialog({
            agents,
            activeWorkspace: undefined,
            dataScope: { kind: "all" },
          }),
        { wrapper }
      );

      act(() => result.current.openDialog("codex-agent"));
      expect(result.current.destinationAxes.project?.value).toBe("ws_alpha");
      await act(async () => result.current.submit());

      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ workspace: "ws_alpha" })
      );
      expect(mockMutateAsync).not.toHaveBeenCalledWith(
        expect.objectContaining({ workspace: "ws_c" })
      );
    });

    // Invariant: a list group's + names its project and profile; the pickers start there.
    it("Should start the pickers on the project and profile the opener named", () => {
      allProjects([activeWorkspace, betaWorkspace]);
      window.localStorage.setItem(LAST_SESSION_PROJECT_STORAGE_KEY, "ws_alpha");
      mockProfileScope.current = { aggregate: true, destination: "default" };
      mockProfiles.current = [
        { name: "default", archived_at: null },
        { name: "marketing", archived_at: null },
      ];
      const { wrapper } = createWrapper();
      const { result } = renderHook(
        () =>
          useSessionCreateDialog({
            agents,
            activeWorkspace: undefined,
            dataScope: { kind: "all" },
          }),
        { wrapper }
      );

      act(() =>
        result.current.openDialog("codex-agent", { project: "ws_beta", profile: "marketing" })
      );
      expect(result.current.destinationAxes.project?.value).toBe("ws_beta");
      expect(result.current.destinationAxes.profile?.value).toBe("marketing");
    });

    it("W-14 Should prefer the last-used project over the first one", () => {
      allProjects([activeWorkspace, betaWorkspace]);
      window.localStorage.setItem(LAST_SESSION_PROJECT_STORAGE_KEY, "ws_beta");
      const { wrapper } = createWrapper();
      const { result } = renderHook(
        () =>
          useSessionCreateDialog({
            agents,
            activeWorkspace: undefined,
            dataScope: { kind: "all" },
          }),
        { wrapper }
      );

      act(() => result.current.openDialog("codex-agent"));
      expect(result.current.destinationAxes.project?.value).toBe("ws_beta");
    });

    it("W-15 Should prefill the acting profile, hide archived ones, and send the pick", async () => {
      mockProfileScope.current = { aggregate: true, destination: "default" };
      mockProfiles.current = [
        { name: "default", archived_at: null },
        { name: "marketing", archived_at: null },
        { name: "old", archived_at: "2026-01-01T00:00:00Z" },
      ];
      mockUseAgents.mockReturnValue({ data: agents });
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => result.current.openDialog("codex-agent"));
      expect(result.current.destinationAxes.project).toBeUndefined();
      expect(result.current.destinationAxes.profile?.value).toBe("default");
      expect(result.current.destinationAxes.profile?.options.map(option => option.value)).toEqual([
        "default",
        "marketing",
      ]);
      act(() => result.current.destinationAxes.profile?.onChange("marketing"));
      expect(mockUseAgents).toHaveBeenLastCalledWith(
        expect.anything(),
        expect.objectContaining({ profile: "marketing" })
      );
      await act(async () => result.current.submit());

      expect(mockMutateAsync).toHaveBeenCalledWith({
        agent_name: "codex-agent",
        workspace: "ws_alpha",
        profile: "marketing",
      });
    });

    it("W-16 Should end the project list with User", () => {
      allProjects([activeWorkspace]);
      const { wrapper } = createWrapper();
      const { result } = renderHook(
        () =>
          useSessionCreateDialog({
            agents,
            activeWorkspace: undefined,
            dataScope: { kind: "all" },
          }),
        { wrapper }
      );

      act(() => result.current.openDialog("codex-agent"));
      expect(result.current.destinationAxes.project?.options.at(-1)).toEqual({
        value: "@user",
        label: "User",
      });
    });

    it("W-16 Should prefill User with zero projects and start the session in User", async () => {
      allProjects([]);
      mockUserHomeDir.current = "/Users/operator";
      mockMutateAsync.mockResolvedValue({
        ...createdSession,
        workspace_id: "",
        workspace_path: "/Users/operator",
      });
      const { wrapper } = createWrapper();
      const { result } = renderHook(
        () =>
          useSessionCreateDialog({
            agents,
            activeWorkspace: undefined,
            dataScope: { kind: "all" },
          }),
        { wrapper }
      );

      act(() => result.current.openDialog("codex-agent"));
      expect(result.current.destinationAxes.project?.options.map(option => option.value)).toEqual([
        "@user",
      ]);
      expect(result.current.destinationAxes.project?.value).toBe("@user");
      expect(result.current.destinationLabel).toBe("User");
      await act(async () => result.current.submit());

      expect(mockMutateAsync).toHaveBeenCalledWith({ agent_name: "codex-agent", scope: "user" });
      await waitFor(() =>
        expect(mockNavigate).toHaveBeenCalledWith(
          expect.objectContaining({ search: { workspace: "@user" } })
        )
      );
    });
  });
});
