// Suite: Onboarding default-model state
// Invariant: provider settings and draft state produce one valid runtime selection.
// Boundary IN: provider, settings, and catalog hook projections.
// Boundary OUT: provider settings persistence and catalog transport.
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockProviders,
  mockSettingsPersona,
  mockSettingsProvider,
  mockUseSettingsPersona,
  mockPutProvider,
  mockUpdatePersona,
  mockCatalogRefresh,
} = vi.hoisted(() => ({
  mockProviders: {
    data: {
      providers: [
        { name: "claude", display_name: "Claude Code", auth_status: { state: "ready" } },
        { name: "codex", display_name: "Codex", auth_status: { state: "ready" } },
      ],
    },
    isLoading: false,
    error: null as Error | null,
  },
  mockSettingsPersona: {
    data: { config: { agent: "general", provider: "", sandbox: "" } },
    error: null as Error | null,
    isSuccess: true,
  },
  mockSettingsProvider: {
    data: {
      settings: {
        display_name: "Claude Code",
        harness: "acp",
        models: { default: "claude-sonnet-5", curated: [] as { id: string }[] },
        credential_slots: [{ name: "api_key", target_env: "ANTHROPIC_API_KEY" }],
      },
    },
    error: null as Error | null,
    isSuccess: true,
  },
  mockUseSettingsPersona: vi.fn(),
  mockPutProvider: { mutateAsync: vi.fn(), isPending: false },
  mockUpdatePersona: { mutateAsync: vi.fn(), isPending: false },
  mockCatalogRefresh: vi.fn(),
}));

vi.mock("@/systems/providers/hooks/use-providers", () => ({
  useProviders: () => mockProviders,
}));

vi.mock("@/systems/model-catalog/hooks/use-runtime-model-catalog", () => ({
  useRuntimeModelCatalog: () => ({
    models: [],
    payloadsByProvider: {},
    loading: false,
    loaded: true,
    error: null,
    stale: false,
    refresh: mockCatalogRefresh,
    refreshing: false,
    refreshError: null,
  }),
}));

vi.mock("@/systems/model-catalog/lib/to-runtime-selector-options", () => ({
  providerNeedsAuth: () => false,
}));

vi.mock("@/systems/settings/hooks/use-settings-collections", () => ({
  useSettingsProvider: () => mockSettingsProvider,
}));

vi.mock("@/systems/settings/hooks/use-settings-mutations", () => ({
  useUpdateSettingsPersona: () => mockUpdatePersona,
  usePutSettingsProvider: () => mockPutProvider,
}));

vi.mock("@/systems/settings/hooks/use-settings-sections", () => ({
  useSettingsPersona: mockUseSettingsPersona,
}));

import { onboardingDraftStore } from "../../stores/use-onboarding-draft-store";
import { useOnboardingDefaultModel } from "../use-onboarding-default-model";

describe("useOnboardingDefaultModel", () => {
  beforeEach(() => {
    window.localStorage.clear();
    onboardingDraftStore.trigger.draftCleared();
    mockPutProvider.mutateAsync.mockReset().mockResolvedValue(undefined);
    mockUpdatePersona.mutateAsync.mockReset().mockResolvedValue(undefined);
    mockUseSettingsPersona.mockReset().mockReturnValue(mockSettingsPersona);
    mockCatalogRefresh.mockReset();
    mockSettingsPersona.data = { config: { agent: "general", provider: "", sandbox: "" } };
    mockSettingsPersona.error = null;
    mockSettingsPersona.isSuccess = true;
    mockSettingsProvider.data = {
      settings: {
        display_name: "Claude Code",
        harness: "acp",
        models: { default: "claude-sonnet-5", curated: [] },
        credential_slots: [{ name: "api_key", target_env: "ANTHROPIC_API_KEY" }],
      },
    };
  });

  it("Should reflect the draft store as the runtime value and provider rail", () => {
    act(() =>
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "claude",
        model: "claude-opus-4-8",
        reasoning: "high",
      })
    );
    const { result } = renderHook(() => useOnboardingDefaultModel());

    expect(result.current.runtimeValue).toEqual({
      provider: "claude",
      model: "claude-opus-4-8",
      reasoning_effort: "high",
    });
    expect(result.current.speed).toBe("normal");
    expect(result.current.runtimeProviders.map(provider => provider.id)).toEqual([
      "claude",
      "codex",
    ]);
  });

  it("Should patch provider/model/reasoning and clear secrets when the provider changes", () => {
    act(() => {
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "claude",
        model: "",
        reasoning: "",
      });
      onboardingDraftStore.trigger.envVarEntered({ envVar: "OLD" });
      onboardingDraftStore.trigger.apiKeyEntered({ apiKey: "sk-old" });
    });
    const { result } = renderHook(() => useOnboardingDefaultModel());

    act(() =>
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.6-sol",
        reasoning_effort: "high",
      })
    );

    const state = onboardingDraftStore.getSnapshot().context;
    expect(state.provider).toBe("codex");
    expect(state.model).toBe("gpt-5.6-sol");
    expect(state.reasoning).toBe("high");
    expect(state.envVar).toBe("");
    expect(state.apiKey).toBe("");
  });

  it("Should accept the selector's normalized speed and expose explicit speed changes", () => {
    const { result } = renderHook(() => useOnboardingDefaultModel());

    act(() =>
      result.current.onRuntimeChange(
        {
          provider: "cursor",
          model: "grok-4.5",
          reasoning_effort: "high",
        },
        "fast"
      )
    );
    expect(result.current.speed).toBe("fast");

    act(() => result.current.onSpeedChange("normal"));
    expect(result.current.speed).toBe("normal");
  });

  it("Should fold the selector's model, reasoning, and speed into the provider settings write on commit", async () => {
    act(() => {
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "claude",
        model: "claude-opus-4-8",
        reasoning: "xhigh",
      });
      onboardingDraftStore.trigger.authModeChosen({ authMode: "native_cli" });
      onboardingDraftStore.trigger.speedSelected({ speed: "fast" });
    });
    const { result } = renderHook(() => useOnboardingDefaultModel());

    await act(async () => {
      await result.current.commit();
    });

    expect(mockPutProvider.mutateAsync).toHaveBeenCalledTimes(1);
    const [request] = mockPutProvider.mutateAsync.mock.calls[0] as [
      {
        name: string;
        body: { settings?: { models?: { default?: string } }; model_curation?: unknown };
      },
    ];
    expect(request.name).toBe("claude");
    expect(request.body.settings?.models?.default).toBe("claude-opus-4-8");
    expect(request.body.model_curation).toEqual({
      model_id: "claude-opus-4-8",
      default_effort: "xhigh",
      default_speed: "fast",
    });
    expect(mockUpdatePersona.mutateAsync).toHaveBeenCalledWith({
      body: { config: { agent: "general", provider: "claude", sandbox: "" } },
      filter: { scope: "user" },
    });
    expect(mockUseSettingsPersona).toHaveBeenCalledWith({ scope: "user" });
  });

  it("Should stay invalid and name profile-default read failures", () => {
    mockSettingsPersona.error = new Error("persona unavailable");
    mockSettingsPersona.isSuccess = false;
    const { result } = renderHook(() => useOnboardingDefaultModel());

    expect(result.current.isValid).toBe(false);
    expect(result.current.configurationError).toBe("persona unavailable");
  });

  it("Should refresh the whole catalog through the aggregate refresh", () => {
    const { result } = renderHook(() => useOnboardingDefaultModel());
    act(() => result.current.onRefreshCatalog());
    expect(mockCatalogRefresh).toHaveBeenCalledTimes(1);
  });

  it("Should default the auth mode to the provider harness until the operator picks", () => {
    act(() =>
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "openrouter",
        model: "",
        reasoning: "",
      })
    );
    mockSettingsProvider.data = {
      settings: {
        display_name: "OpenRouter",
        harness: "pi_acp",
        models: { default: "", curated: [] },
        credential_slots: [],
      },
    };

    const { result } = renderHook(() => useOnboardingDefaultModel());

    expect(result.current.harness).toBe("pi_acp");
    expect(result.current.authMode).toBe("bound_secret");
    // Nothing was written to the draft — the mode is derived, not synced.
    expect(onboardingDraftStore.getSnapshot().context.authMode).toBe("native_cli");
    expect(onboardingDraftStore.getSnapshot().context.authModeTouched).toBe(false);
  });

  it("Should keep the operator's auth choice over the harness default", () => {
    act(() =>
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "openrouter",
        model: "",
        reasoning: "",
      })
    );
    mockSettingsProvider.data = {
      settings: {
        display_name: "OpenRouter",
        harness: "pi_acp",
        models: { default: "", curated: [] },
        credential_slots: [],
      },
    };
    const { result } = renderHook(() => useOnboardingDefaultModel());

    act(() => result.current.onAuthModeChange("native_cli"));

    expect(onboardingDraftStore.getSnapshot().context.authModeTouched).toBe(true);
    expect(result.current.authMode).toBe("native_cli");
  });

  it("Should re-arm the harness default when the provider changes", () => {
    act(() => {
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "claude",
        model: "",
        reasoning: "",
      });
      onboardingDraftStore.trigger.authModeChosen({ authMode: "bound_secret" });
    });
    const { result } = renderHook(() => useOnboardingDefaultModel());
    expect(result.current.authMode).toBe("bound_secret");

    act(() =>
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.6-sol",
        reasoning_effort: "",
      })
    );

    expect(onboardingDraftStore.getSnapshot().context.authModeTouched).toBe(false);
    expect(result.current.authMode).toBe("native_cli");
  });

  it("Should commit the harness-derived mode when the operator never picked one", async () => {
    act(() => {
      onboardingDraftStore.trigger.runtimeSelected({
        provider: "openrouter",
        model: "grok-4",
        reasoning: "",
      });
      onboardingDraftStore.trigger.envVarEntered({ envVar: "OPENROUTER_API_KEY" });
    });
    mockSettingsProvider.data = {
      settings: {
        display_name: "OpenRouter",
        harness: "pi_acp",
        models: { default: "", curated: [] },
        credential_slots: [],
      },
    };
    const { result } = renderHook(() => useOnboardingDefaultModel());

    await act(async () => {
      await result.current.commit();
    });

    const [request] = mockPutProvider.mutateAsync.mock.calls[0] as [
      {
        body: {
          settings?: {
            auth_mode?: string;
            credential_slots?: { name: string; target_env: string }[];
          };
        };
      },
    ];
    expect(request.body.settings?.auth_mode).toBe("bound_secret");
    expect(request.body.settings?.credential_slots?.[0]).toMatchObject({
      name: "api_key",
      target_env: "OPENROUTER_API_KEY",
    });
  });
});
