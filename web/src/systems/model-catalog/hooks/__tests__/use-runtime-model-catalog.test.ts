import { QueryClient, QueryClientProvider, useQueryClient } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/model-catalog-api", async importActual => {
  const actual = await importActual<typeof import("../../adapters/model-catalog-api")>();
  return {
    ...actual,
    listAllModels: vi.fn(),
    refreshAllModels: vi.fn(),
    getModelSourceStatus: vi.fn(),
    refreshProviderModels: vi.fn(),
  };
});

import type { RuntimeCatalogProvider } from "../use-runtime-model-catalog";
import {
  listAllModels,
  getModelSourceStatus,
  refreshProviderModels,
  ModelCatalogApiError,
  refreshAllModels,
} from "../../adapters/model-catalog-api";
import type { ProviderModelPayload } from "../../types";
import { modelCatalogKeys } from "../../lib/query-keys";
import { useRefreshProviderModels } from "../use-refresh-provider-models";
import { useRuntimeModelCatalog } from "../use-runtime-model-catalog";

function payload(
  providerId: string,
  modelId: string,
  overrides: Partial<ProviderModelPayload> = {}
): ProviderModelPayload {
  return {
    provider_id: providerId,
    model_id: modelId,
    availability_state: "available_live",
    available: true,
    curated: true,
    deprecated: false,
    featured: false,
    hidden: false,
    stale: false,
    sources: [],
    ...overrides,
  } as ProviderModelPayload;
}

function createWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

function mockModels(models: ProviderModelPayload[]) {
  vi.mocked(listAllModels).mockResolvedValue({ models } as Awaited<
    ReturnType<typeof listAllModels>
  >);
}

const codexClaude: RuntimeCatalogProvider[] = [{ id: "codex" }, { id: "claude" }];

describe("useRuntimeModelCatalog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: [] });
  });
  afterEach(() => vi.restoreAllMocks());

  it("Should expose an unconfirmed binding diagnostic beside disabled models", async () => {
    const diagnostic = "live catalog unavailable: context deadline exceeded";
    mockModels([
      payload("claude", "claude-sonnet-5", {
        availability_state: "unknown",
        available: false,
        last_error: diagnostic,
      }),
    ]);
    const { result } = renderHook(() => useRuntimeModelCatalog([{ id: "claude" }]), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.loaded).toBe(true));
    expect(result.current.models[0].disabled).toBe(true);
    expect(result.current.providerIssues.claude).toBe(diagnostic);
  });

  it("Should issue one aggregate query and restrict rows to the allow-list in provider order", async () => {
    mockModels([
      payload("codex", "gpt-5.6"),
      payload("openrouter", "leaked"), // outside the surface allow-list → excluded
      payload("claude", "opus-4.8"),
    ]);

    const { result } = renderHook(() => useRuntimeModelCatalog(codexClaude), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.models).toHaveLength(2));
    // Exactly one aggregate request (no per-provider fan-out).
    expect(vi.mocked(listAllModels)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(refreshAllModels)).not.toHaveBeenCalled();
    // Allow-list order preserved (codex before claude); the foreign provider is gone.
    expect(result.current.models.map(model => [model.provider, model.id])).toEqual([
      ["codex", "gpt-5.6"],
      ["claude", "opus-4.8"],
    ]);
    expect(result.current.models.some(model => model.provider === "openrouter")).toBe(false);
  });

  it("Should update a mounted projection when provider permissions or catalog rows change", async () => {
    mockModels([payload("codex", "gpt-5.6"), payload("claude", "opus-4.8")]);
    const { result, rerender } = renderHook(
      ({ providers }: { providers: RuntimeCatalogProvider[] }) => ({
        catalog: useRuntimeModelCatalog(providers),
        client: useQueryClient(),
      }),
      { wrapper: createWrapper(), initialProps: { providers: codexClaude } }
    );
    await waitFor(() => expect(result.current.catalog.models).toHaveLength(2));

    rerender({ providers: [{ id: "claude", needsAuth: true }, { id: "codex" }] });
    expect(result.current.catalog.models.map(model => model.provider)).toEqual(["claude", "codex"]);
    expect(result.current.catalog.models[0].disabled).toBe(true);

    act(() => {
      result.current.client.setQueryData(
        modelCatalogKeys.allModels("all", undefined, undefined, true),
        { models: [payload("codex", "gpt-5.6"), payload("claude", "sonnet-4.7", { stale: true })] }
      );
    });
    await waitFor(() => expect(result.current.catalog.models[0].id).toBe("sonnet-4.7"));
    expect(result.current.catalog.stale).toBe(true);
    expect(result.current.catalog.models[0].disabled).toBe(true);

    rerender({ providers: [{ id: "claude" }] });
    expect(result.current.catalog.models.map(model => model.id)).toEqual(["sonnet-4.7"]);
    expect(result.current.catalog.models[0].disabled).toBe(false);
    expect(vi.mocked(listAllModels)).toHaveBeenCalledTimes(1);
  });

  it("Should refresh and reread once when the initial catalog omits an allowed provider", async () => {
    const codex = payload("codex", "gpt-5.6");
    const cursor = payload("cursor", "grok-4.5", {
      configurations: [
        { reasoning_effort: "low", fast: false },
        { reasoning_effort: "low", fast: true },
        { reasoning_effort: "high", fast: false },
        { reasoning_effort: "high", fast: true },
      ],
    });
    vi.mocked(listAllModels)
      .mockResolvedValueOnce({ models: [codex] } as Awaited<ReturnType<typeof listAllModels>>)
      .mockResolvedValueOnce({ models: [codex, cursor] } as Awaited<
        ReturnType<typeof listAllModels>
      >);
    vi.mocked(refreshAllModels).mockResolvedValue({ sources: [] } as Awaited<
      ReturnType<typeof refreshAllModels>
    >);

    const { result } = renderHook(
      () => useRuntimeModelCatalog([{ id: "codex" }, { id: "cursor" }]),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.models).toHaveLength(2));
    expect(vi.mocked(refreshAllModels)).toHaveBeenCalledTimes(1);
    expect(vi.mocked(listAllModels)).toHaveBeenCalledTimes(2);
    expect(result.current.models[1]).toMatchObject({
      provider: "cursor",
      id: "grok-4.5",
      efforts: ["low", "high"],
      configurations: [
        { reasoning_effort: "low", fast: false },
        { reasoning_effort: "low", fast: true },
        { reasoning_effort: "high", fast: false },
        { reasoning_effort: "high", fast: true },
      ],
    });
  });

  it("Should reread source health when initial readiness fails after an empty status read", async () => {
    mockModels([payload("codex", "gpt-test")]);
    const source = {
      source_id: "provider_live:cursor",
      source_kind: "provider_live",
      provider_id: "cursor",
      priority: 110,
      refresh_state: "failed",
      row_count: 0,
      stale: false,
      failure_reason: "auth_required",
    };
    let rejectRefresh!: (error: Error) => void;
    vi.mocked(refreshAllModels).mockReturnValue(
      new Promise((_resolve, reject) => {
        rejectRefresh = reject;
      })
    );
    const { result } = renderHook(
      () => useRuntimeModelCatalog([{ id: "codex" }, { id: "cursor" }]),
      { wrapper: createWrapper() }
    );
    await waitFor(() => expect(refreshAllModels).toHaveBeenCalledTimes(1));
    expect(result.current.providerIssues).toEqual({});
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: [source] });
    act(() => rejectRefresh(new ModelCatalogApiError("catalog refresh failed", 500, [source])));
    await waitFor(() =>
      expect(result.current.providerIssues.cursor).toBe("Sign in to update the model list.")
    );
    expect(getModelSourceStatus).toHaveBeenCalledTimes(2);
    expect(result.current.refreshError).toBeNull();
    expect(result.current.models[0]?.disabled).toBe(false);
  });

  it("Should force rediscovery and reread the catalog when refreshed by the user", async () => {
    const stale = payload("claude", "claude-opus-5", { stale: true });
    const fresh = payload("claude", "claude-opus-5-5");
    vi.mocked(listAllModels)
      .mockResolvedValueOnce({ models: [stale] } as Awaited<ReturnType<typeof listAllModels>>)
      .mockResolvedValue({
        models: [stale, fresh].map(model => ({ ...model, stale: false })),
      } as Awaited<ReturnType<typeof listAllModels>>);
    vi.mocked(refreshAllModels).mockResolvedValue({ sources: [] } as Awaited<
      ReturnType<typeof refreshAllModels>
    >);

    const { result } = renderHook(() => useRuntimeModelCatalog([{ id: "claude" }]), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.stale).toBe(true));

    act(() => result.current.refresh());

    await waitFor(() => expect(result.current.models).toHaveLength(2));
    expect(vi.mocked(refreshAllModels)).toHaveBeenCalledWith({ force: true });
    expect(result.current.stale).toBe(false);
    expect(result.current.refreshError).toBeNull();
  });

  it("Should surface a failed user refresh instead of a silent no-op", async () => {
    mockModels([payload("claude", "claude-opus-5", { stale: true })]);
    vi.mocked(refreshAllModels).mockRejectedValue(
      new ModelCatalogApiError("inspect claude ACP model options: deadline exceeded", 502)
    );

    const { result } = renderHook(() => useRuntimeModelCatalog([{ id: "claude" }]), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.models).toHaveLength(1));

    act(() => result.current.refresh());

    await waitFor(() =>
      expect(result.current.refreshError).toBe("Could not refresh the model list. Try again.")
    );
  });

  it("Should surface a live source that failed inside a successful forced refresh", async () => {
    mockModels([payload("claude", "claude-opus-5")]);
    const response = {
      sources: [
        {
          source_id: "builtin",
          source_kind: "builtin",
          provider_id: "claude",
          priority: 10,
          refresh_state: "succeeded",
          row_count: 5,
          stale: false,
        },
        {
          source_id: "provider_live:claude",
          source_kind: "provider_live",
          provider_id: "claude",
          priority: 110,
          refresh_state: "failed",
          row_count: 0,
          stale: false,
          last_error: "ACP initialization failed",
        },
        {
          source_id: "provider_live:openrouter",
          source_kind: "provider_live",
          provider_id: "openrouter",
          priority: 110,
          refresh_state: "failed",
          row_count: 0,
          stale: false,
          last_error: "missing key",
        },
      ],
    } as Awaited<ReturnType<typeof refreshAllModels>>;
    vi.mocked(refreshAllModels).mockResolvedValue(response);
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: response.sources });

    const { result } = renderHook(() => useRuntimeModelCatalog([{ id: "claude" }]), {
      wrapper: createWrapper(),
    });
    await waitFor(() => expect(result.current.models).toHaveLength(1));

    act(() => result.current.refresh());

    await waitFor(() =>
      expect(result.current.providerIssues).toEqual({
        claude: "Could not update the model list. Try refreshing again.",
      })
    );
    expect(result.current.refreshError).toBeNull();
    expect(result.current.models.map(model => model.id)).toEqual(["claude-opus-5"]);
  });

  it("Should attribute authentication failures from an HTTP error and preserve healthy and fallback models", async () => {
    mockModels([payload("codex", "gpt-test"), payload("cursor", "cursor-test", { stale: true })]);
    const failure = new ModelCatalogApiError("Could not refresh the model list. Try again.", 500, [
      {
        source_id: "provider_live:cursor",
        source_kind: "provider_live",
        provider_id: "cursor",
        priority: 110,
        refresh_state: "failed",
        row_count: 1,
        stale: true,
        failure_reason: "auth_required",
        last_error: "discovery command failed: Authentication required",
      },
    ]);
    vi.mocked(refreshAllModels).mockRejectedValue(failure);
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: failure.sources });
    const { result } = renderHook(
      () => useRuntimeModelCatalog([{ id: "codex" }, { id: "cursor" }]),
      { wrapper: createWrapper() }
    );
    await waitFor(() => expect(result.current.models).toHaveLength(2));
    act(() => result.current.refresh());
    await waitFor(() =>
      expect(result.current.providerIssues).toEqual({ cursor: "Sign in to update the model list." })
    );
    expect(result.current.refreshError).toBeNull();
    expect(result.current.models.every(model => !model.disabled)).toBe(true);
  });

  it("Should retire refresh guidance after recovery even when a later status reread fails", async () => {
    mockModels([payload("codex", "gpt-test"), payload("cursor", "cursor-test")]);
    const source = {
      source_id: "provider_live:cursor",
      source_kind: "provider_live",
      provider_id: "cursor",
      priority: 110,
      refresh_state: "succeeded",
      row_count: 1,
      stale: false,
    };
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: [source] });
    const wrapper = createWrapper();
    const { result } = renderHook(
      () => useRuntimeModelCatalog([{ id: "codex" }, { id: "cursor" }]),
      { wrapper }
    );
    await waitFor(() => expect(result.current.models).toHaveLength(2));
    expect(result.current.providerIssues).toEqual({});
    vi.mocked(getModelSourceStatus).mockRejectedValue(
      new ModelCatalogApiError("status unavailable", 403)
    );
    vi.mocked(refreshAllModels).mockRejectedValue(
      new ModelCatalogApiError("refresh failed", 500, [
        { ...source, refresh_state: "failed", failure_reason: "auth_required" },
      ])
    );
    act(() => result.current.refresh());
    await waitFor(() =>
      expect(result.current.providerIssues.cursor).toBe("Sign in to update the model list.")
    );
    expect(result.current.refreshError).toBeNull();
    expect(result.current.models.every(model => !model.disabled)).toBe(true);
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: [source] });
    vi.mocked(refreshProviderModels).mockResolvedValue({ sources: [source] });
    const settings = renderHook(
      () => ({ refresh: useRefreshProviderModels(), client: useQueryClient() }),
      { wrapper }
    );
    act(() => settings.result.current.refresh.mutate({ providerId: "cursor", force: true }));
    await waitFor(() => expect(result.current.providerIssues).toEqual({}));
    vi.mocked(getModelSourceStatus).mockRejectedValue(
      new ModelCatalogApiError("later status unavailable", 403)
    );
    await act(() =>
      settings.result.current.client.refetchQueries({ queryKey: modelCatalogKeys.sourceStatus })
    );
    expect(
      settings.result.current.client.getQueryState(modelCatalogKeys.sourceStatus)?.status
    ).toBe("error");
    expect(result.current.providerIssues).toEqual({});
    expect(result.current.models.every(model => !model.disabled)).toBe(true);
  });

  it.each(["successful", "disallowed"])(
    "Should preserve a request error for %s source rows",
    async kind => {
      mockModels([payload("codex", "gpt-test")]);
      const source = {
        source_id: "builtin",
        source_kind: "builtin",
        provider_id: kind === "successful" ? "codex" : "openrouter",
        priority: 10,
        refresh_state: kind === "successful" ? "succeeded" : "failed",
        row_count: 1,
        stale: false,
      };
      vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: [source] });
      vi.mocked(refreshAllModels).mockRejectedValue(
        new ModelCatalogApiError("storage failed", 500, [source])
      );
      const { result } = renderHook(() => useRuntimeModelCatalog([{ id: "codex" }]), {
        wrapper: createWrapper(),
      });
      await waitFor(() => expect(result.current.models).toHaveLength(1));
      act(() => result.current.refresh());
      await waitFor(() =>
        expect(result.current.refreshError).toBe("Could not refresh the model list. Try again.")
      );
      expect(result.current.providerIssues).toEqual({});
    }
  );

  it("Should clear shared provider guidance after a successful refresh from Settings", async () => {
    mockModels([payload("cursor", "cursor-test", { stale: true })]);
    const failed = {
      source_id: "provider_live:cursor",
      source_kind: "provider_live",
      provider_id: "cursor",
      priority: 110,
      refresh_state: "failed",
      row_count: 1,
      stale: true,
      failure_reason: "auth_required",
    };
    vi.mocked(getModelSourceStatus).mockResolvedValue({ sources: [failed] });
    vi.mocked(refreshProviderModels).mockResolvedValue({
      sources: [{ ...failed, refresh_state: "succeeded", failure_reason: undefined }],
    });
    const wrapper = createWrapper();
    const { result } = renderHook(
      () => ({
        catalog: useRuntimeModelCatalog([{ id: "cursor" }]),
        settingsRefresh: useRefreshProviderModels(),
      }),
      { wrapper }
    );
    await waitFor(() =>
      expect(result.current.catalog.providerIssues.cursor).toBe("Sign in to update the model list.")
    );
    vi.mocked(getModelSourceStatus).mockResolvedValue({
      sources: [{ ...failed, refresh_state: "succeeded", failure_reason: undefined }],
    });
    act(() => result.current.settingsRefresh.mutate({ providerId: "cursor", force: true }));
    await waitFor(() => expect(result.current.catalog.providerIssues).toEqual({}));
    expect(result.current.catalog.refreshError).toBeNull();
    expect(result.current.catalog.models[0]?.disabled).toBe(false);
    const reopened = renderHook(() => useRuntimeModelCatalog([{ id: "cursor" }]), { wrapper });
    await waitFor(() => expect(reopened.result.current.providerIssues).toEqual({}));
  });

  it("Should disable every row of a needs-auth provider", async () => {
    mockModels([payload("codex", "gpt-5.6")]);

    const { result } = renderHook(
      () => useRuntimeModelCatalog([{ id: "codex", needsAuth: true }]),
      { wrapper: createWrapper() }
    );

    await waitFor(() => expect(result.current.models).toHaveLength(1));
    expect(result.current.models[0]?.disabled).toBe(true);
    expect(result.current.models[0]?.disabled_reason).toBe("Sign in");
  });

  it("Should project a stale source as stale", async () => {
    mockModels([
      payload("codex", "gpt-5.6", { availability_state: "available_stale", stale: true }),
    ]);

    const { result } = renderHook(() => useRuntimeModelCatalog([{ id: "codex" }]), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.models).toHaveLength(1));
    expect(result.current.stale).toBe(true);
  });

  it("Should expose the loading state while the aggregate query is pending", () => {
    vi.mocked(listAllModels).mockReturnValue(new Promise(() => undefined));

    const { result } = renderHook(() => useRuntimeModelCatalog(codexClaude), {
      wrapper: createWrapper(),
    });

    expect(result.current.loading).toBe(true);
    expect(result.current.models).toEqual([]);
  });

  it("Should expose the error message when the aggregate query fails", async () => {
    // A 403 is non-retryable (shouldRetry), so the error state settles immediately.
    vi.mocked(listAllModels).mockRejectedValue(new ModelCatalogApiError("catalog exploded", 403));

    const { result } = renderHook(() => useRuntimeModelCatalog(codexClaude), {
      wrapper: createWrapper(),
    });

    await waitFor(() => expect(result.current.error).toBe("catalog exploded"));
  });
});
