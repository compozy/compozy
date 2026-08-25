import { act, renderHook, waitFor } from "@testing-library/react";
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

vi.mock("@/systems/settings/hooks/use-settings-collections", () => ({
  useSettingsProviders: () => mockSettingsProviders,
}));

vi.mock("@/systems/model-catalog/hooks/use-runtime-model-catalog", () => ({
  useRuntimeModelCatalog: () => ({ ...mockRuntimeCatalog, refresh: mockRefreshCatalog }),
}));

vi.mock("@/systems/model-catalog/lib/to-runtime-selector-options", () => ({
  providerNeedsAuth: () => false,
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
      activeWorkspace: "activeWorkspace" in overrides ? overrides.activeWorkspace : activeWorkspace,
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
        scope: "workspace",
        workspace: "ws_alpha",
        agent: {
          name: "release-captain",
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
      });
      expect(mockNavigate).toHaveBeenCalledWith({
        to: "/agents/$name",
        params: { name: "release-captain" },
      });
      expect(result.current.open).toBe(false);
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
      reasoningEffort: "ultra",
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
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      model: "gpt-5.4",
      tools: ["compozy__skill_view"],
      description: "",
      scope: "workspace",
      shadowed: false,
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
    const { result } = renderAgentCreateDialog();
    const source = {
      name: "release-captain",
      provider: "codex",
      prompt: "Own release readiness.",
      model: "gpt-5.4",
      tools: ["compozy__skill_view"],
      description: "",
      scope: "workspace",
      shadowed: false,
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
