import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, render, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AgentPayload } from "@/systems/agent";
import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import type { AllModelsListResponse, AllModelsRefreshResponse } from "@/systems/model-catalog";
import type { WorkspaceDetailPayload, WorkspacePayload } from "@/systems/workspace";

import type { SessionPayload } from "../../types";
import { useSessionCreateDialog, type SessionCreateDialogApi } from "../use-session-create-dialog";

type ProviderModelPayload = AllModelsListResponse["models"][number];

const visibleCatalogFlags = {
  curated: true,
  deprecated: false,
  featured: false,
  hidden: false,
} satisfies Pick<ProviderModelPayload, "curated" | "deprecated" | "featured" | "hidden">;

const {
  mockNavigate,
  mockMutateAsync,
  mockToastError,
  mockUseCreateSessionPending,
  mockWorkspaceQuery,
  mockWorkspaceListRef,
  mockUseAgents,
  mockListAllModels,
  mockRefreshAllModels,
} = vi.hoisted(() => ({
  mockNavigate: vi.fn<(input: unknown) => Promise<void>>(),
  mockMutateAsync: vi.fn<(input: unknown) => Promise<SessionPayload>>(),
  mockToastError: vi.fn(),
  mockUseCreateSessionPending: { current: false as boolean },
  mockWorkspaceQuery: vi.fn(),
  mockWorkspaceListRef: { current: [] as WorkspacePayload[] },
  mockUseAgents: vi.fn(),
  mockListAllModels: vi.fn<(input: unknown) => Promise<AllModelsListResponse>>(),
  mockRefreshAllModels: vi.fn<(input: unknown) => Promise<AllModelsRefreshResponse>>(),
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock("sonner", () => ({
  toast: {
    error: mockToastError,
  },
}));

vi.mock("@/systems/agent", async () => {
  const actual = await vi.importActual<typeof import("@/systems/agent")>("@/systems/agent");

  return {
    ...actual,
    useAgents: (workspaceId: string, options?: { enabled?: boolean }) =>
      mockUseAgents(workspaceId, options),
  };
});

vi.mock("@/systems/workspace", async () => {
  const actual = await vi.importActual<typeof import("@/systems/workspace")>("@/systems/workspace");

  return {
    ...actual,
    useWorkspace: (workspaceId: string, options?: { enabled?: boolean }) =>
      mockWorkspaceQuery(workspaceId, options),
    useWorkspaces: () => ({
      data: mockWorkspaceListRef.current,
      error: null,
      isLoading: false,
    }),
  };
});

vi.mock("@/systems/model-catalog/adapters/model-catalog-api", async () => {
  const actual = await vi.importActual<
    typeof import("@/systems/model-catalog/adapters/model-catalog-api")
  >("@/systems/model-catalog/adapters/model-catalog-api");
  return {
    ...actual,
    listAllModels: (...args: unknown[]) => mockListAllModels(args[0]),
    refreshAllModels: (...args: unknown[]) => mockRefreshAllModels(args[0]),
  };
});

vi.mock("../use-session-actions", () => ({
  useCreateSession: () => ({
    mutateAsync: mockMutateAsync,
    isPending: mockUseCreateSessionPending.current,
  }),
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

const agentsWithDefaultModel: AgentPayload[] = [
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
    model: "gpt-5.5",
    prompt: "code",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

const agentsWithInheritedRuntime: AgentPayload[] = [
  {
    name: "inherited-agent",
    provider: "",
    prompt: "code",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
    effective_runtime: {
      provider: "codex",
      model: "gpt-5.4",
      reasoning_effort: "high",
      sources: {
        provider: "project_default",
        model: "provider_default",
        reasoning_effort: "model_default",
      },
    },
  },
];

const agentsWithUnavailableInheritedRuntime: AgentPayload[] = [
  {
    ...agentsWithInheritedRuntime[0],
    name: "unavailable-agent",
    effective_runtime: {
      ...agentsWithInheritedRuntime[0].effective_runtime!,
      provider: "missing-provider",
    },
  },
];

const agentsWithCursorAlias: AgentPayload[] = [
  ...agents,
  {
    name: "cursor-agent",
    provider: "cursor",
    model: "cursor-grok-4.5-high",
    prompt: "edit",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

const createdSession: SessionPayload = {
  id: "sess-new",
  agent_name: "codex-agent",
  provider: "codex",
  workspace_id: "ws_alpha",
  workspace_path: "/workspace/alpha",
  state: "active",
  badge: "idle",
  attachable: true,
  available_commands: [],
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:01Z",
};

let workspaceQueryResult: {
  data: WorkspaceDetailPayload | undefined;
  isLoading: boolean;
  error: Error | null;
};

const codexCatalog: AllModelsListResponse = {
  models: [
    {
      ...visibleCatalogFlags,
      provider_id: "codex",
      model_id: "gpt-5.4",
      display_name: "GPT-5.4",
      availability_state: "available_live",
      available: true,
      stale: false,
      refreshed_at: "2026-05-07T10:00:00Z",
      sources: [
        {
          source_id: "config",
          source_kind: "config",
          priority: 120,
          refreshed_at: "2026-05-07T10:00:00Z",
          stale: false,
        },
      ],
      supports_reasoning: true,
      reasoning_efforts: ["low", "medium", "high"],
      default_reasoning_effort: "medium",
    },
    {
      ...visibleCatalogFlags,
      provider_id: "codex",
      model_id: "gpt-5.4-mini",
      display_name: "GPT-5.4 Mini",
      availability_state: "available_stale",
      available: true,
      stale: true,
      refreshed_at: "2026-05-06T10:00:00Z",
      sources: [
        {
          source_id: "models_dev",
          source_kind: "models_dev",
          priority: 50,
          refreshed_at: "2026-05-06T10:00:00Z",
          stale: true,
        },
      ],
      supports_reasoning: false,
    },
  ],
};

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);

  return { queryClient, wrapper };
}

const FIRST_MESSAGE = "Draft the release notes.";

async function submitWithPrompt(
  result: { current: SessionCreateDialogApi },
  prompt: string = FIRST_MESSAGE
) {
  act(() => {
    result.current.onPromptChange(prompt);
  });
  await act(async () => {
    await result.current.submit();
  });
}

describe("useSessionCreateDialog", () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockNavigate.mockResolvedValue(undefined);
    mockMutateAsync.mockReset();
    mockMutateAsync.mockResolvedValue(createdSession);
    mockToastError.mockReset();
    mockWorkspaceQuery.mockReset();
    mockWorkspaceListRef.current = [activeWorkspace];
    mockUseAgents.mockReset();
    mockUseAgents.mockReturnValue({ data: undefined });
    mockUseCreateSessionPending.current = false;

    workspaceQueryResult = {
      data: {
        workspace: activeWorkspace,
        providers: [{ name: "claude" }, { name: "codex" }, { name: "cursor" }, { name: "gemini" }],
      },
      isLoading: false,
      error: null,
    };

    mockWorkspaceQuery.mockImplementation(() => workspaceQueryResult);
    mockListAllModels.mockReset();
    mockListAllModels.mockResolvedValue(codexCatalog);
    mockRefreshAllModels.mockReset();
    mockRefreshAllModels.mockResolvedValue({
      sources: [
        {
          source_id: "models_dev",
          source_kind: "models_dev",
          priority: 50,
          provider_id: "codex",
          refresh_state: "succeeded",
          row_count: 2,
          stale: false,
        },
      ],
    });
  });

  it("Should derive the default provider once workspace providers arrive after opening", async () => {
    workspaceQueryResult = {
      data: {
        workspace: activeWorkspace,
        providers: [],
      },
      isLoading: true,
      error: null,
    };

    const { wrapper } = createWrapper();
    const { result, rerender } = renderHook(
      () => useSessionCreateDialog({ agents, activeWorkspace }),
      { wrapper }
    );

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    expect(result.current.selectedAgentName).toBe("codex-agent");
    expect(result.current.runtimeValue.provider).toBe("");

    workspaceQueryResult = {
      data: {
        workspace: activeWorkspace,
        providers: [{ name: "claude" }, { name: "codex" }, { name: "gemini" }],
      },
      isLoading: false,
      error: null,
    };

    rerender();

    expect(result.current.runtimeValue.provider).toBe("codex");

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
    });
    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/agents/$name/sessions/$id",
      params: { name: "codex-agent", id: "sess-new" },
    });
  });

  it("Should resolve a relative working path without sending a workspace reference", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openForAgent("claude-agent"));
    act(() => {
      result.current.onSessionNameChange("  Investigate checkout latency  ");
      result.current.onWorkspacePathChange("  services/checkout  ");
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "claude-agent",
      name: "Investigate checkout latency",
      workspace_path: "/workspace/alpha/services/checkout",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
    });
  });

  it("Should reject a working path that escapes the selected workspace", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openForAgent("claude-agent"));
    act(() => result.current.onWorkspacePathChange("../other-workspace"));
    await submitWithPrompt(result);

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe(
      "Working path must stay within the selected workspace."
    );
  });

  it("Should omit name, working path, and runtime overrides while they are untouched", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openForAgent("claude-agent"));
    await submitWithPrompt(result);

    const payload = mockMutateAsync.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("name");
    expect(payload).not.toHaveProperty("workspace_path");
    expect(payload).not.toHaveProperty("provider");
    expect(payload).not.toHaveProperty("model");
    expect(payload).not.toHaveProperty("reasoning_effort");
  });

  it("Should snap advanced-only selections back to a Simple-valid default when leaving Advanced", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openForAgent("claude-agent"));
    act(() => result.current.onModeChange("advanced"));
    act(() => {
      result.current.onWorkspacePathChange("services/checkout");
      result.current.onNetworkParticipationChange({
        mode: "live",
        channelId: "",
        channelStrategy: "named",
      });
    });

    // A live participation draft with no channel is invalid — leaving Advanced
    // must not strand the operator with a blocking value they can no longer see.
    expect(result.current.networkParticipation.mode).toBe("live");

    act(() => result.current.onModeChange("simple"));

    expect(result.current.mode).toBe("simple");
    expect(result.current.workspacePath).toBe("");
    expect(result.current.networkParticipation).toEqual({
      mode: "local",
      channelId: "",
      channelStrategy: "",
    });
  });

  it("Should retarget the launch when a different workspace is selected", () => {
    const otherWorkspace = { ...activeWorkspace, id: "ws_beta", name: "beta" };
    mockWorkspaceListRef.current = [activeWorkspace, otherWorkspace];
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openForAgent("claude-agent"));
    expect(result.current.workspaceId).toBe("ws_alpha");

    act(() => result.current.onWorkspaceChange("ws_beta"));

    expect(result.current.workspaceId).toBe("ws_beta");
    expect(result.current.workspace?.id).toBe("ws_beta");
    // Agents and providers are workspace-scoped, so the agent pick cannot carry over.
    expect(result.current.selectedAgentName).toBe("");
  });

  it("Should replace the agent population with the selected workspace query", () => {
    const betaWorkspace = { ...activeWorkspace, id: "ws_beta", name: "beta" };
    const betaAgents: AgentPayload[] = [
      {
        name: "beta-agent",
        provider: "codex",
        prompt: "work in beta",
        origin: "workspace",
        workspace_id: betaWorkspace.id,
        definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
      },
    ];
    mockWorkspaceListRef.current = [activeWorkspace, betaWorkspace];
    mockUseAgents.mockImplementation((workspaceId: string) => ({
      data: workspaceId === betaWorkspace.id ? betaAgents : undefined,
    }));
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => result.current.openForAgent("claude-agent"));
    expect(result.current.agents.map(agent => agent.name)).toEqual(["claude-agent", "codex-agent"]);

    act(() => result.current.onWorkspaceChange(betaWorkspace.id));

    expect(mockUseAgents).toHaveBeenLastCalledWith(betaWorkspace.id, { enabled: true });
    expect(result.current.agents.map(agent => agent.name)).toEqual(["beta-agent"]);
  });

  it("Should keep an unavailable inherited provider unresolved instead of displaying an unsent fallback", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () =>
        useSessionCreateDialog({
          agents: agentsWithUnavailableInheritedRuntime,
          activeWorkspace,
        }),
      { wrapper }
    );

    act(() => {
      result.current.openForAgent("unavailable-agent");
    });

    await waitFor(() => {
      expect(result.current.providersLoading).toBe(false);
    });
    expect(result.current.runtimeValue.provider).toBe("");
    expect(result.current.providersError).toContain('Provider "missing-provider" is not available');

    await submitWithPrompt(result);

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe("Choose a provider configured for this workspace.");

    act(() => {
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.4",
        reasoning_effort: "high",
      });
    });
    expect(result.current.providersError).toBeNull();

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "unavailable-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "gpt-5.4",
      reasoning_effort: "high",
    });
  });

  it("Should display effective project runtime while preserving inheritance on create", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useSessionCreateDialog({ agents: agentsWithInheritedRuntime, activeWorkspace }),
      { wrapper }
    );

    act(() => {
      result.current.openForAgent("inherited-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels).toHaveLength(2);
    });
    expect(result.current.runtimeValue).toEqual({
      provider: "codex",
      model: "gpt-5.4",
      reasoning_effort: "high",
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "inherited-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
    });
  });

  it("Should release the creation dialog before destination navigation settles", async () => {
    let resolveNavigation: (() => void) | undefined;
    const navigation = new Promise<void>(resolve => {
      resolveNavigation = resolve;
    });
    let blockingDialogPresentWhenNavigationStarted: boolean | undefined;
    let composerPresentWhenNavigationStarted: boolean | undefined;
    let dialog: SessionCreateDialogApi | undefined;

    const { wrapper } = createWrapper();
    const Harness = () => {
      dialog = useSessionCreateDialog({ agents, activeWorkspace });
      return dialog.open
        ? createElement("div", { role: "dialog" }, "Starting session")
        : createElement("textarea", { "aria-label": "Session composer", readOnly: true });
    };
    const getDialog = () => {
      if (!dialog) throw new Error("Session create dialog harness did not render");
      return dialog;
    };

    render(createElement(Harness), { wrapper });
    mockNavigate.mockImplementation(() => {
      blockingDialogPresentWhenNavigationStarted =
        document.querySelector('[role="dialog"]') !== null;
      composerPresentWhenNavigationStarted =
        document.querySelector('[aria-label="Session composer"]') !== null;
      return navigation;
    });

    act(() => {
      getDialog().openForAgent("codex-agent");
    });
    act(() => {
      getDialog().onPromptChange(FIRST_MESSAGE);
    });

    let submitSettled = false;
    let submitPromise: Promise<void> | undefined;
    act(() => {
      submitPromise = getDialog()
        .submit()
        .then(() => {
          submitSettled = true;
        });
    });

    try {
      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith({
          to: "/agents/$name/sessions/$id",
          params: { name: "codex-agent", id: "sess-new" },
        });
      });

      expect(blockingDialogPresentWhenNavigationStarted).toBe(false);
      expect(composerPresentWhenNavigationStarted).toBe(true);
      expect(getDialog().open).toBe(false);
      await waitFor(() => {
        expect(submitSettled).toBe(true);
      });
      expect(getDialog().pendingAgentName).toBeNull();
      expect(getDialog().pendingWorkspaceId).toBeNull();
    } finally {
      await act(async () => {
        resolveNavigation?.();
        await submitPromise;
      });
    }
  });

  it("Should map workspace providers onto the runtime rail options", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    expect(result.current.hasProviderOptions).toBe(true);
    expect(result.current.runtimeProviders.map(option => option.id)).toEqual([
      "claude",
      "codex",
      "cursor",
      "gemini",
    ]);
  });

  it("Should clear an explicit provider override when the operator changes agents", () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("claude-agent");
    });

    expect(result.current.runtimeValue.provider).toBe("claude");

    act(() => {
      result.current.onRuntimeChange({ provider: "gemini", model: "", reasoning_effort: "" });
    });

    expect(result.current.runtimeValue.provider).toBe("gemini");

    act(() => {
      result.current.onAgentChange("codex-agent");
    });

    expect(result.current.selectedAgentName).toBe("codex-agent");
    expect(result.current.runtimeValue.provider).toBe("codex");
  });

  describe("first message", () => {
    it("Should stage the trimmed first message on the single creation request", async () => {
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("codex-agent");
      });
      await submitWithPrompt(result, `  \n${FIRST_MESSAGE}\n  `);

      expect(mockMutateAsync).toHaveBeenCalledTimes(1);
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ prompt: FIRST_MESSAGE })
      );
    });

    it("Should refuse to create a session from a blank first message", async () => {
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("codex-agent");
      });
      await submitWithPrompt(result, "   \n  ");

      expect(mockMutateAsync).not.toHaveBeenCalled();
      expect(mockNavigate).not.toHaveBeenCalled();
      expect(result.current.submitError).toBe(
        "Write the first message before starting the session."
      );
    });

    it("Should preserve the draft for a retry when creation fails", async () => {
      mockMutateAsync.mockRejectedValueOnce(new Error("daemon unavailable"));
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("codex-agent");
      });
      act(() => {
        result.current.onRuntimeChange({
          provider: "codex",
          model: "gpt-5.4",
          reasoning_effort: "high",
        });
      });
      await submitWithPrompt(result);

      expect(result.current.submitError).toBe("daemon unavailable");
      expect(result.current.open).toBe(true);
      expect(result.current.promptValue).toBe(FIRST_MESSAGE);
      expect(result.current.runtimeValue.model).toBe("gpt-5.4");
    });

    it("Should submit only once when two actions reenter before mutation state updates", async () => {
      let resolveCreate: ((session: SessionPayload) => void) | undefined;
      mockMutateAsync.mockImplementationOnce(
        () =>
          new Promise<SessionPayload>(resolve => {
            resolveCreate = resolve;
          })
      );
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("codex-agent");
        result.current.onPromptChange(FIRST_MESSAGE);
      });

      let firstSubmit: Promise<void> | undefined;
      let secondSubmit: Promise<void> | undefined;
      act(() => {
        firstSubmit = result.current.submit();
        secondSubmit = result.current.submit();
      });
      expect(mockMutateAsync).toHaveBeenCalledTimes(1);

      await act(async () => {
        resolveCreate?.(createdSession);
        await Promise.all([firstSubmit, secondSubmit]);
      });
    });

    it("Should clear the draft only after the daemon durably accepts the session", async () => {
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("codex-agent");
      });
      await submitWithPrompt(result);

      expect(mockMutateAsync).toHaveBeenCalledTimes(1);
      expect(result.current.promptValue).toBe("");
      expect(result.current.open).toBe(false);
    });

    it("Should keep the typed message when the operator switches agents", () => {
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("claude-agent");
      });
      act(() => {
        result.current.onPromptChange(FIRST_MESSAGE);
      });
      act(() => {
        result.current.onRuntimeChange({ provider: "gemini", model: "", reasoning_effort: "" });
      });
      act(() => {
        result.current.onAgentChange("codex-agent");
      });

      expect(result.current.promptValue).toBe(FIRST_MESSAGE);
      expect(result.current.runtimeValue.provider).toBe("codex");
    });

    it("Should preserve prompt and runtime when the dialog is closed and reopened for the same agent", async () => {
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("claude-agent");
      });
      await waitFor(() => {
        expect(result.current.runtimeModels.length).toBeGreaterThan(0);
      });
      act(() => {
        result.current.onPromptChange(FIRST_MESSAGE);
      });
      act(() => {
        result.current.onRuntimeChange({
          provider: "codex",
          model: "gpt-5.4",
          reasoning_effort: "high",
        });
      });
      act(() => {
        result.current.onOpenChange(false);
      });
      act(() => {
        result.current.openForAgent("claude-agent");
      });

      expect(result.current.open).toBe(true);
      expect(result.current.promptValue).toBe(FIRST_MESSAGE);
      expect(result.current.runtimeValue).toEqual({
        provider: "codex",
        model: "gpt-5.4",
        reasoning_effort: "high",
      });
    });

    it("Should reset agent-derived runtime and Network when reopened in a different workspace", () => {
      const otherWorkspace: WorkspacePayload = { ...activeWorkspace, id: "ws_beta", name: "beta" };
      const { wrapper } = createWrapper();
      const { result, rerender } = renderHook(
        ({ workspace }: { workspace: WorkspacePayload }) =>
          useSessionCreateDialog({ agents, activeWorkspace: workspace }),
        { wrapper, initialProps: { workspace: activeWorkspace } }
      );

      act(() => {
        result.current.openForAgent("claude-agent");
      });
      act(() => {
        result.current.onPromptChange(FIRST_MESSAGE);
      });
      act(() => {
        result.current.onRuntimeChange({ provider: "gemini", model: "", reasoning_effort: "" });
      });
      act(() => {
        result.current.onNetworkParticipationChange({
          mode: "live",
          channelId: "alpha-release-room",
          channelStrategy: "named",
        });
      });
      act(() => {
        result.current.onOpenChange(false);
      });

      rerender({ workspace: otherWorkspace });
      act(() => {
        result.current.openForAgent("claude-agent");
      });

      expect(result.current.promptValue).toBe("");
      expect(result.current.runtimeValue.provider).toBe("claude");
      expect(result.current.runtimeValue.model).toBe("");
      expect(result.current.networkParticipation).toEqual({
        mode: "local",
        channelId: "",
        channelStrategy: "",
      });
    });

    it("Should preserve the message but reset runtime when reopened for a different agent", () => {
      const { wrapper } = createWrapper();
      const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
        wrapper,
      });

      act(() => {
        result.current.openForAgent("claude-agent");
      });
      act(() => {
        result.current.onPromptChange(FIRST_MESSAGE);
      });
      act(() => {
        result.current.onRuntimeChange({ provider: "gemini", model: "", reasoning_effort: "" });
      });
      act(() => {
        result.current.onOpenChange(false);
      });
      act(() => {
        result.current.openForAgent("codex-agent");
      });

      expect(result.current.promptValue).toBe(FIRST_MESSAGE);
      expect(result.current.runtimeValue.provider).toBe("codex");
    });
  });

  it("Should expose deduped runtime models for the selected provider using the all view", async () => {
    mockListAllModels.mockResolvedValueOnce({
      models: [
        codexCatalog.models[0],
        codexCatalog.models[1],
        codexCatalog.models[0],
      ] as AllModelsListResponse["models"],
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels).toHaveLength(2);
    });
    expect(result.current.runtimeModels.map(option => option.id)).toEqual([
      "gpt-5.4",
      "gpt-5.4-mini",
    ]);
    // Single aggregate `view=all` request across providers — never per-provider.
    expect(mockListAllModels).toHaveBeenCalledWith(
      expect.objectContaining({ includeStale: true, view: "all" })
    );
  });

  it("Should reject a non-catalog model for an authoritative provider when the catalog is empty", async () => {
    mockListAllModels.mockResolvedValueOnce({ models: [] });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.catalogLoading).toBe(false);
    });
    expect(result.current.runtimeModels).toEqual([]);

    act(() => {
      result.current.onRuntimeChange({
        provider: "cursor",
        model: "cursor-grok-4.5-high",
        reasoning_effort: "",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toContain("not in the selected provider catalog");

    act(() => {
      result.current.onNetworkParticipationChange({
        mode: "live",
        channelId: "release-room",
        channelStrategy: "named",
      });
    });
    expect(result.current.submitError).toBeNull();
  });

  it("Should preserve a free-form model for a non-authoritative provider when the catalog is empty", async () => {
    mockListAllModels.mockResolvedValueOnce({ models: [] });

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.catalogLoading).toBe(false);
    });

    act(() => {
      result.current.onRuntimeChange({
        provider: "codex",
        model: "custom-experimental",
        reasoning_effort: "",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "custom-experimental",
    });
    expect(result.current.submitError).toBeNull();
  });

  it("Should expose stale catalog rows without blocking session creation", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels).toHaveLength(2);
    });

    expect(result.current.catalogStale).toBe(true);
    const staleOption = result.current.runtimeModels.find(option => option.id === "gpt-5.4-mini");
    expect(staleOption?.availability).toBe("stale");
    const liveOption = result.current.runtimeModels.find(option => option.id === "gpt-5.4");
    expect(liveOption?.availability).toBe("live");

    act(() => {
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.4-mini",
        reasoning_effort: "",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "gpt-5.4-mini",
    });
  });

  it("Should reject an explicit model for an authoritative provider when the catalog is unavailable", async () => {
    mockListAllModels.mockReset();
    mockListAllModels.mockRejectedValue(new Error("catalog upstream failed"));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(
      () => {
        expect(result.current.catalogError).toContain("catalog upstream failed");
      },
      { timeout: 5000 }
    );
    expect(result.current.runtimeModels).toEqual([]);

    act(() => {
      result.current.onRuntimeChange({
        provider: "cursor",
        model: "cursor-grok-4.5-high",
        reasoning_effort: "",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toContain("Model catalog is unavailable");
  });

  it("Should preserve a free-form model for a non-authoritative provider when the catalog is unavailable", async () => {
    mockListAllModels.mockReset();
    mockListAllModels.mockRejectedValue(new Error("catalog upstream failed"));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(
      () => {
        expect(result.current.catalogError).toContain("catalog upstream failed");
      },
      { timeout: 5000 }
    );

    act(() => {
      result.current.onRuntimeChange({
        provider: "codex",
        model: "manual-fallback",
        reasoning_effort: "",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "manual-fallback",
    });
    expect(result.current.submitError).toBeNull();
  });

  it("Should preserve the provider-native model default when the catalog is unavailable", async () => {
    mockListAllModels.mockReset();
    mockListAllModels.mockRejectedValue(new Error("catalog upstream failed"));

    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(
      () => {
        expect(result.current.catalogError).toContain("catalog upstream failed");
      },
      { timeout: 5000 }
    );

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
    });
  });

  it("Should reject an inherited model alias before session mutation", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useSessionCreateDialog({ agents: agentsWithCursorAlias, activeWorkspace }),
      { wrapper }
    );

    act(() => {
      result.current.openForAgent("cursor-agent");
    });

    await waitFor(() => {
      expect(result.current.catalogLoaded).toBe(true);
    });
    expect(result.current.runtimeValue.model).toBe("cursor-grok-4.5-high");

    await submitWithPrompt(result);

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(result.current.submitError).toContain("not in the selected provider catalog");
  });

  it("Should invalidate catalog queries on refresh", async () => {
    const { queryClient, wrapper } = createWrapper();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels).toHaveLength(2);
    });

    act(() => {
      result.current.refreshCatalog();
    });

    await waitFor(() => {
      // The refresh affordance truthfully refreshes the whole catalog.
      expect(mockRefreshAllModels).toHaveBeenCalledWith({});
    });
    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalled();
    });
  });

  it("Should thread the model and reasoning override when the model supports reasoning", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels.length).toBeGreaterThan(0);
    });

    act(() => {
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.4",
        reasoning_effort: "high",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenLastCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "gpt-5.4",
      reasoning_effort: "high",
    });
  });

  it("Should omit the reasoning override when the selected model lacks reasoning support", async () => {
    const { wrapper } = createWrapper();
    const { result } = renderHook(() => useSessionCreateDialog({ agents, activeWorkspace }), {
      wrapper,
    });

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels.length).toBeGreaterThan(0);
    });

    act(() => {
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.4-mini",
        reasoning_effort: "high",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenLastCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "gpt-5.4-mini",
    });
  });

  it("Should submit a coherent runtime override when reasoning changes", async () => {
    mockListAllModels.mockResolvedValueOnce({
      models: [
        ...codexCatalog.models,
        {
          ...visibleCatalogFlags,
          provider_id: "codex",
          model_id: "gpt-5.5",
          display_name: "GPT-5.5",
          availability_state: "available_live",
          available: true,
          stale: false,
          refreshed_at: "2026-05-07T10:00:00Z",
          sources: [
            {
              source_id: "models_dev",
              source_kind: "models_dev",
              priority: 50,
              refreshed_at: "2026-05-07T10:00:00Z",
              stale: false,
            },
          ],
          supports_reasoning: true,
          reasoning_efforts: ["minimal", "low", "medium", "high", "xhigh"],
          default_reasoning_effort: "medium",
        },
      ],
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useSessionCreateDialog({ agents: agentsWithDefaultModel, activeWorkspace }),
      { wrapper }
    );

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(result.current.runtimeModels.some(option => option.id === "gpt-5.5")).toBe(true);
    });
    // The inherited agent-default model is RENDERED so its reasoning is reachable…
    expect(result.current.runtimeValue.model).toBe("gpt-5.5");

    act(() => {
      // …and the selector emits the effective model with the chosen effort.
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.5",
        reasoning_effort: "high",
      });
    });

    await submitWithPrompt(result);

    // A selector change becomes one coherent override so provider/model/reasoning
    // cannot be interpreted against different defaults during session startup.
    expect(mockMutateAsync).toHaveBeenCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "codex",
      model: "gpt-5.5",
      reasoning_effort: "high",
    });
  });

  it("Should send the model when the same model id is chosen under a different provider", async () => {
    // The same canonical id exists under BOTH codex and claude. The agent default
    // is codex/gpt-5.5; picking gpt-5.5 under CLAUDE must ship the model with the
    // chosen provider (compound identity), never silently inherit the codex default.
    const sharedModel = {
      ...visibleCatalogFlags,
      model_id: "gpt-5.5",
      display_name: "GPT-5.5",
      availability_state: "available_live",
      available: true,
      stale: false,
      refreshed_at: "2026-05-07T10:00:00Z",
      sources: [
        {
          source_id: "config",
          source_kind: "config",
          priority: 120,
          refreshed_at: "2026-05-07T10:00:00Z",
          stale: false,
        },
      ],
      supports_reasoning: false,
    };
    mockListAllModels.mockResolvedValueOnce({
      models: [
        ...codexCatalog.models,
        { ...sharedModel, provider_id: "codex" },
        { ...sharedModel, provider_id: "claude" },
      ],
    });
    const { wrapper } = createWrapper();
    const { result } = renderHook(
      () => useSessionCreateDialog({ agents: agentsWithDefaultModel, activeWorkspace }),
      { wrapper }
    );

    act(() => {
      result.current.openForAgent("codex-agent");
    });

    await waitFor(() => {
      expect(
        result.current.runtimeModels.some(
          option => option.id === "gpt-5.5" && option.provider === "claude"
        )
      ).toBe(true);
    });

    act(() => {
      result.current.onRuntimeChange({
        provider: "claude",
        model: "gpt-5.5",
        reasoning_effort: "",
      });
    });

    await submitWithPrompt(result);

    expect(mockMutateAsync).toHaveBeenLastCalledWith({
      agent_name: "codex-agent",
      workspace: "ws_alpha",
      prompt: FIRST_MESSAGE,
      network_participation: { mode: "local" },
      provider: "claude",
      model: "gpt-5.5",
    });
  });
});
