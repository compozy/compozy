import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../adapters/model-catalog-api", async importActual => {
  const actual = await importActual<typeof import("../../adapters/model-catalog-api")>();
  return { ...actual, refreshAllModels: vi.fn() };
});

import { refreshAllModels } from "../../adapters/model-catalog-api";
import { modelCatalogKeys } from "../../lib/query-keys";
import { useRefreshAllModels } from "../use-refresh-all-models";

function setup() {
  const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries").mockResolvedValue(undefined);
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return { invalidateSpy, wrapper };
}

describe("useRefreshAllModels", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(refreshAllModels).mockResolvedValue({ sources: [] } as Awaited<
      ReturnType<typeof refreshAllModels>
    >);
  });
  afterEach(() => vi.restoreAllMocks());

  it("Should refresh the aggregate catalog and invalidate the whole catalog cache", async () => {
    const { invalidateSpy, wrapper } = setup();
    const { result } = renderHook(() => useRefreshAllModels(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ force: true });
    });

    expect(vi.mocked(refreshAllModels)).toHaveBeenCalledWith({ force: true });
    await waitFor(() =>
      expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: modelCatalogKeys.all })
    );
  });

  it("Should refresh with an empty input when invoked without arguments", async () => {
    const { wrapper } = setup();
    const { result } = renderHook(() => useRefreshAllModels(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync();
    });

    expect(vi.mocked(refreshAllModels)).toHaveBeenCalledWith({});
  });
});
