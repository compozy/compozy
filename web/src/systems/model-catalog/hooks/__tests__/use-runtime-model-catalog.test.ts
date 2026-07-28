import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/model-catalog-api", async importActual => {
  const actual = await importActual<typeof import("../../adapters/model-catalog-api")>();
  return { ...actual, listAllModels: vi.fn(), refreshAllModels: vi.fn() };
});

import type { RuntimeCatalogProvider } from "../use-runtime-model-catalog";
import { listAllModels, ModelCatalogApiError } from "../../adapters/model-catalog-api";
import type { ProviderModelPayload } from "../../types";
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
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.restoreAllMocks());

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
    // Allow-list order preserved (codex before claude); the foreign provider is gone.
    expect(result.current.models.map(model => [model.provider, model.id])).toEqual([
      ["codex", "gpt-5.6"],
      ["claude", "opus-4.8"],
    ]);
    expect(result.current.models.some(model => model.provider === "openrouter")).toBe(false);
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
