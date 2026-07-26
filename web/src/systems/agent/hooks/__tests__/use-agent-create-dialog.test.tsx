import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockCreateAgent,
  mockDuplicateAgent,
  mockNavigate,
  mockSettingsProviders,
  mockToastError,
  mockRuntimeCatalog,
  mockRefreshCatalog,
} = vi.hoisted(() => ({
  mockCreateAgent: vi.fn(),
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

vi.mock("@/systems/settings", () => ({
  useSettingsProviders: () => mockSettingsProviders,
}));

vi.mock("@/systems/model-catalog", () => ({
  providerNeedsAuth: () => false,
  useRuntimeModelCatalog: () => ({ ...mockRuntimeCatalog, refresh: mockRefreshCatalog }),
}));

vi.mock("../use-agents", () => ({
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
  return renderHook(() =>
    useAgentCreateDialog({
      activeWorkspace: overrides.activeWorkspace ?? activeWorkspace,
      workspaceProviders: overrides.workspaceProviders ?? workspaceProviders,
      workspaceProvidersError: overrides.workspaceProvidersError ?? null,
      workspaceProvidersLoading: overrides.workspaceProvidersLoading ?? false,
    })
  );
}

describe("useAgentCreateDialog", () => {
  beforeEach(() => {
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

  it("uses global settings-backed providers after switching scope", () => {
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({ ...result.current.draft, scope: "global", provider: "" });
    });

    // The runtime provider option surfaces the settings display name (display_name → name).
    expect(result.current.providerOptions[0]?.name).toBe("Claude Code");
    expect(result.current.providerOptions[0]?.harness).toBe("acp");
    expect(result.current.providerOptions[0]?.runtime_provider).toBe("claude");
  });

  it("submits a workspace create request and navigates to the created agent", async () => {
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
        tools: ["agh__skill_view"],
        toolsets: ["agh__catalog"],
        denyTools: ["agh__task_*"],
        disabledSkills: ["copywriting"],
        permissions: "approve-reads",
      });
    });

    await act(async () => {
      await result.current.onSubmit();
    });

    expect(mockCreateAgent).toHaveBeenCalledWith({
      scope: "workspace",
      workspace: "ws_alpha",
      agent: {
        name: "release-captain",
        provider: "codex",
        prompt: "Own release readiness.",
        model: "gpt-5.4",
        reasoning_effort: "high",
        tools: ["agh__skill_view"],
        toolsets: ["agh__catalog"],
        deny_tools: ["agh__task_*"],
        permissions: "approve-reads",
        category_path: ["Engineering", "Release"],
        skills: { disabled: ["copywriting"] },
      },
    });
    expect(mockNavigate).toHaveBeenCalledWith({
      to: "/agents/$name",
      params: { name: "release-captain" },
    });
    expect(result.current.open).toBe(false);
  });

  it("fails closed and keeps the dialog open when a corrupt draft carries an off-contract reasoning effort", async () => {
    const { result } = renderAgentCreateDialog();

    // Simulate a corrupt/foreign draft carrying an off-contract effort. It must
    // fail closed BEFORE any wire call — never silently stripped and submitted. A
    // compile-time type alone can't protect the wire from untrusted persisted state.
    const corruptDraft = {
      ...result.current.draft,
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      reasoningEffort: "ultra",
    };
    act(() => {
      result.current.openDialog();
      result.current.onDraftChange(corruptDraft as typeof result.current.draft);
    });

    await act(async () => {
      await result.current.onSubmit();
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

    await act(async () => {
      await result.current.onSubmit();
    });

    expect(result.current.open).toBe(true);
    expect(result.current.submitError).toBe("agent definition already exists");
    expect(mockToastError).toHaveBeenCalledWith("agent definition already exists");
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("blocks global submit when global providers fail to load", async () => {
    mockSettingsProviders.data = undefined;
    mockSettingsProviders.error = new Error("Unable to load global provider settings.");
    const { result } = renderAgentCreateDialog();

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({
        ...result.current.draft,
        scope: "global",
        name: "global-reviewer",
        provider: "claude",
        prompt: "Review global work.",
      });
    });

    await act(async () => {
      await result.current.onSubmit();
    });

    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe("Unable to load global provider settings.");
  });

  it("does not submit workspace-scoped agents when no active workspace is available", async () => {
    const { result } = renderAgentCreateDialog({
      activeWorkspace: undefined,
      workspaceProviders: [],
    });

    act(() => {
      result.current.openDialog();
      result.current.onDraftChange({
        ...result.current.draft,
        scope: "workspace",
        name: "release-captain",
        provider: "claude",
        prompt: "Own release readiness.",
      });
    });

    await act(async () => {
      await result.current.onSubmit();
    });

    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(result.current.submitError).toBe("No providers are configured for this scope.");
  });

  it("Should openForDuplicate with prefilled draft and hard-reset on openDialog", () => {
    const { result } = renderAgentCreateDialog();
    const source = {
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      model: "gpt-5.4",
      tools: ["agh__skill_view"],
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
    expect(result.current.draft.tools).toEqual(["agh__skill_view"]);
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
    const { result } = renderAgentCreateDialog();
    const source = {
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      model: "gpt-5.4",
      tools: ["agh__skill_view"],
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

    await act(async () => {
      await result.current.onSubmit();
    });

    expect(result.current.submitError).toBeNull();
    expect(mockCreateAgent).not.toHaveBeenCalled();
    expect(mockDuplicateAgent).toHaveBeenCalledWith({
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
