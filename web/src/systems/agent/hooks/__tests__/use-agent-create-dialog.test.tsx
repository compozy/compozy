import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { PropsWithChildren } from "react";
import { resetProfileViews, setProfileView } from "@/systems/profiles";

import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockCreateAgent,
  mockCreateInstance,
  mockCapacity,
  mockDuplicateAgent,
  mockNavigate,
  mockSettingsProviders,
  mockToastError,
  mockRuntimeCatalog,
  mockRefreshCatalog,
} = vi.hoisted(() => ({
  mockCreateAgent: vi.fn(),
  mockCreateInstance: vi.fn(),
  mockCapacity: { current: { limit: 10, active: 0, reached: false } },
  mockDuplicateAgent: vi.fn(),
  mockNavigate: vi.fn(),
  mockSettingsProviders: {
    data: {
      providers: [
        {
          name: "claude",
          settings: {
            display_name: "Claude Code",
            harness: "acp",
            runtime_provider: "claude",
          },
        },
      ],
    } as
      | {
          providers: Array<{
            name: string;
            settings: {
              display_name?: string;
              harness?: string;
              runtime_provider?: string;
            };
          }>;
        }
      | undefined,
    isLoading: false,
    isFetching: false,
    error: null as Error | null,
  },
  mockToastError: vi.fn(),
  mockRefreshCatalog: vi.fn(),
  mockRuntimeCatalog: {
    models: [] as unknown[],
    payloadsByProvider: {} as Record<string, unknown[]>,
    loading: false,
    error: null as string | null,
    stale: false,
    refreshing: false,
    refreshError: null as string | null,
  },
}));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock("sonner", () => ({
  toast: {
    error: mockToastError,
  },
}));

vi.mock("@/systems/settings/hooks/use-settings-collections", () => ({
  useSettingsProviders: () => mockSettingsProviders,
}));

vi.mock("@/systems/model-catalog/hooks/use-runtime-model-catalog", () => ({
  useRuntimeModelCatalog: () => ({ ...mockRuntimeCatalog, refresh: mockRefreshCatalog }),
}));

vi.mock("@/systems/model-catalog/lib/to-runtime-selector-options", () => ({
  providerNeedsAuth: () => false,
}));

vi.mock("../use-agent-instances", () => ({
  useAgentInstanceCapacity: () => mockCapacity.current,
  useCreateAgentInstance: () => ({ mutateAsync: mockCreateInstance }),
}));

vi.mock("../use-agents", () => ({
  useAgents: () => ({ data: [] }),
  useCreateAgent: () => ({
    mutateAsync: mockCreateAgent,
    isPending: false,
  }),
  useDuplicateAgent: () => ({
    mutateAsync: mockDuplicateAgent,
    isPending: false,
  }),
}));

import { useAgentCreateDialog } from "../use-agent-create-dialog";

const activeWorkspace = {
  id: "ws_alpha",
  root_dir: "/workspace/alpha",
  add_dirs: [],
  name: "alpha",
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:00Z",
};

const workspaceProviders = [
  {
    name: "codex",
    display_name: "Codex",
    harness: "acp",
    runtime_provider: "codex",
  },
];

function renderAgentCreateDialog(
  overrides: Partial<{
    activeWorkspace: typeof activeWorkspace | undefined;
    workspaceProviders: typeof workspaceProviders;
    workspaceProvidersError: string | null;
    workspaceProvidersLoading: boolean;
  }> = {}
) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: PropsWithChildren) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(
    () =>
      useAgentCreateDialog({
        activeWorkspace:
          "activeWorkspace" in overrides ? overrides.activeWorkspace : activeWorkspace,
        workspaceProviders: overrides.workspaceProviders ?? workspaceProviders,
        workspaceProvidersError: overrides.workspaceProvidersError ?? null,
        workspaceProvidersLoading: overrides.workspaceProvidersLoading ?? false,
      }),
    { wrapper }
  );
}

describe("useAgentCreateDialog", () => {
  afterEach(() => act(() => resetProfileViews()));
  beforeEach(() => {
    resetProfileViews();
    mockCapacity.current = { limit: 10, active: 0, reached: false };
    mockCreateInstance.mockReset();
    mockCreateInstance.mockResolvedValue({});
    mockCreateAgent.mockReset();
    mockCreateAgent.mockResolvedValue({
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
    });
    mockDuplicateAgent.mockReset();
    mockDuplicateAgent.mockResolvedValue({
      name: "release-captain-copy",
      provider: "codex",
      prompt: "Own release readiness.",
    });
    mockNavigate.mockReset();
    mockNavigate.mockResolvedValue(undefined);
    mockToastError.mockReset();
    mockSettingsProviders.data = {
      providers: [
        {
          name: "claude",
          settings: {
            display_name: "Claude Code",
            harness: "acp",
            runtime_provider: "claude",
          },
        },
      ],
    };
    mockSettingsProviders.isLoading = false;
    mockSettingsProviders.isFetching = false;
    mockSettingsProviders.error = null;
    mockRefreshCatalog.mockReset();
    mockRuntimeCatalog.models = [];
    mockRuntimeCatalog.payloadsByProvider = {};
    mockRuntimeCatalog.loading = false;
    mockRuntimeCatalog.error = null;
    mockRuntimeCatalog.stale = false;
    mockRuntimeCatalog.refreshing = false;
    mockRuntimeCatalog.refreshError = null;
  });

  it("defaults creation to the active workspace", () => {
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog();
    });

    expect(result.current.open).toBe(true);
    expect(result.current.draft.scope).toBe("workspace");
    // Identity is the provider id; name now surfaces the workspace display name.
    expect(result.current.providerOptions.map(option => option.id)).toEqual(["codex"]);
    expect(result.current.providerOptions[0]?.name).toBe("Codex");
  });

  it("uses global settings-backed providers while the menubar scope is Global", () => {
    // Scope is menubar-owned: Global means no active workspace is bound.
    const { result } = renderAgentCreateDialog({ activeWorkspace: undefined });

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({ ...result.current.draft, provider: "" });
    });

    // The runtime provider option surfaces the settings display name (display_name → name).
    expect(result.current.providerOptions[0]?.name).toBe("Claude Code");
    expect(result.current.providerOptions[0]?.harness).toBe("acp");
    expect(result.current.providerOptions[0]?.runtime_provider).toBe("claude");
  });

  it("submits a workspace create request and navigates to the created agent", async () => {
    setProfileView({ kind: "profile", profile: "open-design" });
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({
        ...result.current.draft,
        name: "release-captain",
        categoryPath: "Engineering/Release",
        provider: "codex",
        model: "gpt-5.4",
        reasoningEffort: "high",
        prompt: "Own release readiness.",
        tools: ["compozy__skill_view"],
        toolsets: ["compozy__catalog"],
        denyTools: ["compozy__task_*"],
        disabledSkills: ["copywriting"],
        permissions: "approve-reads",
      });
    });

    act(() => {
      result.current.onSubmit();
    });

    await waitFor(() => {
      expect(mockCreateAgent).toHaveBeenCalledWith({
        profile: "open-design",
        params: {
          scope: "workspace",
          workspace: "ws_alpha",
          agent: {
            name: "release-captain",
            kind: "on_demand",
            appearance: expect.objectContaining({
              version: 1,
              avatar: "generated",
              seed: "release-captain",
              generation: 1,
              color: expect.stringMatching(/^#[0-9a-f]{6}$/i),
            }),
            provider: "codex",
            prompt: "Own release readiness.",
            model: "gpt-5.4",
            reasoning_effort: "high",
            tools: ["compozy__skill_view"],
            toolsets: ["compozy__catalog"],
            deny_tools: ["compozy__task_*"],
            permissions: "approve-reads",
            category_path: ["Engineering", "Release"],
            skills: { disabled: ["copywriting"] },
          },
        },
      });
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name",
        params: { name: "release-captain" },
      });
      expect(result.current.open).toBe(false);
    });
  });

  function fillStaysOn(
    result: { current: ReturnType<typeof useAgentCreateDialog> },
    place: string
  ) {
    act(() => {
      result.current.openDialog({ place });
    });
    act(() => {
      result.current.onDraftChange({
        ...result.current.draft,
        displayName: "Release Captain",
        description: "Keeps the release moving",
        prompt: "Own release readiness.",
      });
    });
    act(() => {
      result.current.onSubmit();
    });
  }

  it("Should open on Stays on with the scope chosen from a picker's Create new", () => {
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog({ place: "ws_beta" });
    });

    expect(result.current.draft).toMatchObject({ kind: "stays_on", place: "ws_beta" });
    // A Stays on definition lives at the user level, so any project can start it.
    expect(result.current.draft.scope).toBe("global");
  });

  it("Should create at the user level, start the first instance and return to sessions", async () => {
    const { result } = renderAgentCreateDialog();

    fillStaysOn(result, "ws_alpha");

    await waitFor(() => {
      expect(mockCreateInstance).toHaveBeenCalledWith({
        agent: "release-captain",
        workspace: "ws_alpha",
      });
      expect(mockNavigate).toHaveBeenCalledWith({ to: "/sessions" });
    });
    expect(mockCreateAgent.mock.calls[0]?.[0].params).toMatchObject({
      scope: "global",
      agent: {
        kind: "stays_on",
        display_name: "Release Captain",
        description: "Keeps the release moving",
      },
    });
  });

  it("Should create no instance for Decide later", async () => {
    const { result } = renderAgentCreateDialog();

    fillStaysOn(result, "later");

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name",
        params: { name: "release-captain" },
      });
    });
    expect(mockCreateInstance).not.toHaveBeenCalled();
  });

  it("Should create the agent unstarted at the limit", async () => {
    mockCapacity.current = { limit: 10, active: 10, reached: true };
    const { result } = renderAgentCreateDialog();

    fillStaysOn(result, "ws_alpha");

    await waitFor(() => expect(mockCreateAgent).toHaveBeenCalledOnce());
    await waitFor(() => expect(mockNavigate).toHaveBeenCalled());
    expect(mockCreateInstance).not.toHaveBeenCalled();
  });

  it("Should keep the agent and report it when the first start fails", async () => {
    mockCreateInstance.mockRejectedValue(new Error("This agent can't reach that workspace."));
    const { result } = renderAgentCreateDialog();

    fillStaysOn(result, "ws_alpha");

    await waitFor(() =>
      expect(result.current.startFailure).toBe("This agent can't reach that workspace.")
    );
    expect(result.current.open).toBe(true);
    expect(mockNavigate).not.toHaveBeenCalled();

    act(() => {
      result.current.onOpenCreatedAgent();
    });

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name",
        params: { name: "release-captain" },
      });
    });
  });

  it("Should stay closed after a committed create fails to navigate", async () => {
    mockNavigate.mockRejectedValue(new Error("router unavailable"));
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({
        ...result.current.draft,
        name: "release-captain",
        provider: "codex",
        prompt: "Own release readiness.",
      });
    });
    act(() => {
      result.current.onSubmit();
    });

    await waitFor(() => {
      expect(mockToastError).toHaveBeenCalledWith("router unavailable");
      expect(result.current.open).toBe(false);
    });
    expect(mockCreateAgent).toHaveBeenCalledOnce();

    act(() => {
      result.current.openDialog();
    });
    expect(result.current.open).toBe(true);
    expect(mockCreateAgent).toHaveBeenCalledOnce();
  });

  it("fails closed and keeps the dialog open when a corrupt draft carries an off-contract reasoning effort", () => {
    const { result } = renderAgentCreateDialog();

    // Simulate a corrupt/foreign draft carrying an off-contract effort. It must
    // fail closed BEFORE any wire call — never silently stripped and submitted. A
    // compile-time type alone can't protect the wire from untrusted persisted state.
    const corruptDraft = {
      ...result.current.draft,
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      reasoningEffort: "invalid effort",
    };
    act(() => {
      result.current.openDialog();
      result.current.onDraftChange(corruptDraft as typeof result.current.draft);
    });

    act(() => {
      result.current.onSubmit();
    });

    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(result.current.open).toBe(true);
    expect(result.current.submitError).toBe("Choose a valid reasoning effort.");
  });

  it("keeps the dialog open and reports submit failures", async () => {
    mockCreateAgent.mockRejectedValue(new Error("agent definition already exists"));
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({
        ...result.current.draft,
        name: "release-captain",
        provider: "codex",
        prompt: "Own release readiness.",
      });
    });

    act(() => {
      result.current.onSubmit();
    });

    await waitFor(() => {
      expect(result.current.open).toBe(true);
      expect(result.current.submitError).toBe("agent definition already exists");
      expect(mockToastError).toHaveBeenCalledWith("agent definition already exists");
      expect(mockNavigate).not.toHaveBeenCalled();
    });
  });

  it("blocks global submit when global providers fail to load", () => {
    mockSettingsProviders.data = undefined;
    mockSettingsProviders.error = new Error("Unable to load global provider settings.");
    const { result } = renderAgentCreateDialog({ activeWorkspace: undefined });

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({
        ...result.current.draft,
        name: "global-reviewer",
        provider: "claude",
        prompt: "Review global work.",
      });
    });

    act(() => {
      result.current.onSubmit();
    });

    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe("Unable to load global provider settings.");
  });

  it("Should openForDuplicate with prefilled draft and hard-reset on openDialog", () => {
    const { result } = renderAgentCreateDialog();
    const source = {
      label: "Release Captain",
      kind: "on_demand",
      resolved_appearance: {
        version: 1,
        avatar: "generated",
        seed: "release-captain",
        generation: 1,
        color: "#7c5cff",
      },
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      model: "gpt-5.4",
      tools: ["compozy__skill_view"],
      origin: "workspace" as const,
      definition_digest: "d".repeat(64),
      category_path: ["Engineering", "Release"],
    };

    act(() => {
      result.current.openForDuplicate(source);
    });

    expect(result.current.open).toBe(true);
    expect(result.current.mode).toBe("duplicate");
    expect(result.current.duplicateSourceName).toBe("release-captain");
    expect(result.current.draft.name).toBe("");
    expect(result.current.draft.provider).toBe("codex");
    expect(result.current.draft.prompt).toBe("Own release readiness.");
    expect(result.current.draft.tools).toEqual(["compozy__skill_view"]);
    expect(result.current.draft.categoryPath).toBe("Engineering/Release");

    act(() => {
      result.current.openDialog();
    });

    expect(result.current.mode).toBe("create");
    expect(result.current.duplicateSourceName).toBeNull();
    expect(result.current.draft.name).toBe("");
    expect(result.current.draft.provider).toBe("");
    expect(result.current.draft.prompt).toBe("");
  });

  it("Should submit duplicate override diff and never call create", async () => {
    setProfileView({ kind: "profile", profile: "open-design" });
    const { result } = renderAgentCreateDialog();
    const source = {
      label: "Release Captain",
      kind: "on_demand",
      resolved_appearance: {
        version: 1,
        avatar: "generated",
        seed: "release-captain",
        generation: 1,
        color: "#7c5cff",
      },
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      model: "gpt-5.4",
      tools: ["compozy__skill_view"],
      origin: "workspace" as const,
      definition_digest: "d".repeat(64),
    };

    act(() => {
      result.current.openForDuplicate(source);
    });

    act(() => {
      result.current.onDraftChange({
        ...result.current.draft,
        name: "release-captain-copy",
        prompt: "Own release readiness, carefully.",
      });
    });

    act(() => {
      result.current.onSubmit();
    });

    await waitFor(() => {
      expect(result.current.submitError).toBeNull();
      expect(mockCreateAgent).not.toHaveBeenCalled();
      expect(mockDuplicateAgent).toHaveBeenCalledWith({
        profile: "open-design",
        sourceName: "release-captain",
        params: {
          name: "release-captain-copy",
          scope: "workspace",
          workspace: "ws_alpha",
          overrides: {
            prompt: "Own release readiness, carefully.",
          },
        },
      });
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name",
        params: { name: "release-captain-copy" },
      });
    });
  });
});
