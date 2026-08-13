import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useSelector } from "@xstate/store-react";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";

import type { SessionPayload } from "../../types";
import {
  useSessionCreateDialogController,
  useSessionCreateDialogViewModel,
  type SessionCreateDialogApi,
} from "../use-session-create-dialog";
import type { AgentPayload } from "@/systems/agent";
import type { WorkspacePayload } from "@/systems/workspace";

// Invariant: creation submits only durable session identity and launch context, then hands off
// to the canonical session route. Owning layer: session-create view model. Canonical suite: this hook test.
const {
  mockMutateAsync,
  mockNavigate,
  mockSetActiveWorkspaceId,
  mockUseAgents,
  mockUserHomeDir,
  mockWorkspaceListRef,
} = vi.hoisted(() => ({
  mockMutateAsync: vi.fn<(input: unknown) => Promise<SessionPayload>>(),
  mockNavigate: vi.fn<(input: unknown) => Promise<void>>(),
  mockSetActiveWorkspaceId: vi.fn<(workspaceId: string | null) => void>(),
  mockUseAgents: vi.fn(),
  mockUserHomeDir: { current: undefined as string | undefined },
  mockWorkspaceListRef: { current: [] as WorkspacePayload[] },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mockNavigate }));

vi.mock("@/systems/agent/hooks/use-agents", () => ({
  useAgents: (workspaceId: string, options?: { enabled?: boolean }) =>
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

const activeWorkspace: WorkspacePayload = {
  id: "ws_alpha",
  root_dir: "/workspace/alpha",
  add_dirs: [],
  name: "alpha",
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:00Z",
};

const agents: AgentPayload[] = [
  {
    name: "claude-agent",
    provider: "claude",
    prompt: "help",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: "codex-agent",
    provider: "codex",
    prompt: "code",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

const createdSession: SessionPayload = {
  id: "sess-new",
  agent_name: "codex-agent",
  workspace_id: "ws_alpha",
  workspace_path: "/workspace/alpha",
  state: "active",
  badge: "idle",
  attachable: true,
  archived_at: null,
  available_commands: [],
  runtime: { status: "unbound", selection_revision: 0 },
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:01Z",
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { wrapper };
}

function useSessionCreateDialog(context: {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
  scope?: "workspace" | "global";
}): SessionCreateDialogApi & { openDialog: (agentName: string) => void } {
  const controller = useSessionCreateDialogController();
  const dialog = useSessionCreateDialogViewModel(context, controller.store);
  return {
    ...dialog,
    openDialog: agentName => {
      if (!context.activeWorkspace) return;
      controller.store.trigger.dialogOpened({
        agentName,
        workspaceId: context.activeWorkspace.id,
      });
    },
  };
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
    expect(result.current.secondDraftAgentName).toBe("");
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
      network_participation: { mode: "local" },
    });
    await waitFor(() =>
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name/sessions/$id",
        params: { name: "codex-agent", id: "sess-new" },
      })
    );
    expect(mockSetActiveWorkspaceId).not.toHaveBeenCalled();
    expect(result.current.restoreFocusOnClose).toBe(false);
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

  it("Should not activate a workspace after a Global session create", async () => {
    mockUserHomeDir.current = "/Users/operator";
    mockMutateAsync.mockResolvedValue({
      ...createdSession,
      workspace_id: "ws_home",
      workspace_path: "/Users/operator",
    });

    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useSessionCreateDialog({ agents, activeWorkspace, scope: "global" }),
      { wrapper }
    );

    act(() => result.current.openDialog("codex-agent"));
    await act(async () => result.current.submit());

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
});
