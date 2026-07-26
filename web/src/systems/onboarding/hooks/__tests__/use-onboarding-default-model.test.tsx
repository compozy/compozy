import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockProviders,
  mockSettingsGeneral,
  mockSettingsProvider,
  mockPutProvider,
  mockUpdateGeneral,
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
  mockSettingsGeneral: {
    data: { config: { defaults: {} as Record<string, string> } },
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
  mockPutProvider: { mutateAsync: vi.fn(), isPending: false },
  mockUpdateGeneral: { mutateAsync: vi.fn(), isPending: false },
  mockCatalogRefresh: vi.fn(),
}));

vi.mock("@/systems/providers", () => ({ useProviders: () => mockProviders }));

vi.mock("@/systems/model-catalog", () => ({
  providerNeedsAuth: () => false,
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

vi.mock("@/systems/settings", () => ({
  useSettingsGeneral: () => mockSettingsGeneral,
  useSettingsProvider: () => mockSettingsProvider,
  useUpdateSettingsGeneral: () => mockUpdateGeneral,
  usePutSettingsProvider: () => mockPutProvider,
}));

import { useOnboardingDraftStore } from "../../stores/use-onboarding-draft-store";
import { useOnboardingDefaultModel } from "../use-onboarding-default-model";

describe("useOnboardingDefaultModel", () => {
  beforeEach(() => {
    window.localStorage.clear();
    useOnboardingDraftStore.getState().reset();
    mockPutProvider.mutateAsync.mockReset().mockResolvedValue(undefined);
    mockUpdateGeneral.mutateAsync.mockReset().mockResolvedValue(undefined);
    mockCatalogRefresh.mockReset();
    mockSettingsGeneral.data = { config: { defaults: {} } };
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
      useOnboardingDraftStore
        .getState()
        .patch({ provider: "claude", model: "claude-opus-4-8", reasoning: "high" })
    );
    const { result } = renderHook(() => useOnboardingDefaultModel());

    expect(result.current.runtimeValue).toEqual({
      provider: "claude",
      model: "claude-opus-4-8",
      reasoning_effort: "high",
    });
    expect(result.current.runtimeProviders.map(provider => provider.id)).toEqual([
      "claude",
      "codex",
    ]);
    expect(result.current.catalogLoaded).toBe(true);
  });

  it("Should patch provider/model/reasoning and clear secrets when the provider changes", () => {
    act(() =>
      useOnboardingDraftStore
        .getState()
        .patch({ provider: "claude", envVar: "OLD", apiKey: "sk-old" })
    );
    const { result } = renderHook(() => useOnboardingDefaultModel());

    act(() =>
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.6-sol",
        reasoning_effort: "high",
      })
    );

    const state = useOnboardingDraftStore.getState();
    expect(state.provider).toBe("codex");
    expect(state.model).toBe("gpt-5.6-sol");
    expect(state.reasoning).toBe("high");
    expect(state.envVar).toBe("");
    expect(state.apiKey).toBe("");
  });

  it("Should fold the selector's model and reasoning into the provider settings write on commit", async () => {
    act(() =>
      useOnboardingDraftStore.getState().patch({
        provider: "claude",
        model: "claude-opus-4-8",
        reasoning: "xhigh",
        authMode: "native_cli",
      })
    );
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
    });
    expect(mockUpdateGeneral.mutateAsync).toHaveBeenCalledWith({
      config: { defaults: { provider: "claude" } },
    });
  });

  it("Should refresh the whole catalog through the aggregate refresh", () => {
    const { result } = renderHook(() => useOnboardingDefaultModel());
    act(() => result.current.onRefreshCatalog());
    expect(mockCatalogRefresh).toHaveBeenCalledTimes(1);
  });

  it("Should default the auth mode to the provider harness until the operator picks", () => {
    act(() => useOnboardingDraftStore.getState().patch({ provider: "openrouter" }));
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
    expect(useOnboardingDraftStore.getState().authMode).toBe("native_cli");
    expect(useOnboardingDraftStore.getState().authModeTouched).toBe(false);
  });

  it("Should keep the operator's auth choice over the harness default", () => {
    act(() => useOnboardingDraftStore.getState().patch({ provider: "openrouter" }));
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

    expect(useOnboardingDraftStore.getState().authModeTouched).toBe(true);
    expect(result.current.authMode).toBe("native_cli");
  });

  it("Should re-arm the harness default when the provider changes", () => {
    act(() =>
      useOnboardingDraftStore
        .getState()
        .patch({ provider: "claude", authMode: "bound_secret", authModeTouched: true })
    );
    const { result } = renderHook(() => useOnboardingDefaultModel());
    expect(result.current.authMode).toBe("bound_secret");

    act(() =>
      result.current.onRuntimeChange({
        provider: "codex",
        model: "gpt-5.6-sol",
        reasoning_effort: "",
      })
    );

    expect(useOnboardingDraftStore.getState().authModeTouched).toBe(false);
    expect(result.current.authMode).toBe("native_cli");
  });

  it("Should commit the harness-derived mode when the operator never picked one", async () => {
    act(() =>
      useOnboardingDraftStore
        .getState()
        .patch({ provider: "openrouter", model: "grok-4", envVar: "OPENROUTER_API_KEY" })
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
