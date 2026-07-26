import { UIProvider } from "@agh/ui";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ProviderModelCatalogStatus } from "../provider-model-catalog-status";

const { mockRefreshMutation, mockStatusQuery } = vi.hoisted(() => ({
  mockRefreshMutation: {
    error: null as Error | null,
    isPending: false,
    mutate: vi.fn(),
  },
  mockStatusQuery: {
    data: {
      sources: [
        {
          source_id: "models.dev",
          source_kind: "models_dev",
          refresh_state: "succeeded",
          row_count: 42,
          stale: true,
        },
      ],
    },
    error: null as Error | null,
    isFetching: false,
    isLoading: false,
  },
}));

vi.mock("@/systems/model-catalog", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/model-catalog")>();
  return {
    ...actual,
    useProviderModelStatus: () => mockStatusQuery,
    useRefreshProviderModels: () => mockRefreshMutation,
  };
});

function renderStatus() {
  return render(
    <UIProvider reducedMotion="always">
      <ProviderModelCatalogStatus providerId="codex" testId="provider-catalog" />
    </UIProvider>
  );
}

describe("ProviderModelCatalogStatus", () => {
  beforeEach(() => {
    mockRefreshMutation.error = null;
    mockRefreshMutation.isPending = false;
    mockRefreshMutation.mutate.mockReset();
    mockStatusQuery.data = {
      sources: [
        {
          source_id: "models.dev",
          source_kind: "models_dev",
          refresh_state: "succeeded",
          row_count: 42,
          stale: true,
        },
      ],
    };
    mockStatusQuery.error = null;
    mockStatusQuery.isFetching = false;
    mockStatusQuery.isLoading = false;
  });

  it("Should render catalog source rows through Item slots", () => {
    const { container } = renderStatus();

    const row = screen.getByTestId("provider-catalog-source-models.dev");
    expect(row).toHaveAttribute("data-slot", "item");
    expect(row).toHaveTextContent("models.dev");
    expect(row).toHaveTextContent("succeeded");
    expect(row).toHaveTextContent("stale");
    expect(screen.getByTestId("provider-catalog-source-models.dev-rows")).toHaveTextContent(
      "42 rows"
    );
    // SUT_IS_CORRECT_BECAUSE ItemGroup is the public list primitive, so its
    // catalog rows must expose native list semantics to assistive technology.
    const list = container.querySelector("ul");
    expect(list).toBeInTheDocument();
    expect(row.closest("li")?.parentElement).toBe(list);
  });

  it("Should request a forced refresh for the current provider", async () => {
    const user = userEvent.setup();
    renderStatus();

    await user.click(screen.getByTestId("provider-catalog-refresh"));

    expect(mockRefreshMutation.mutate).toHaveBeenCalledWith({ providerId: "codex", force: true });
  });

  it("Should render loading and empty states", () => {
    mockStatusQuery.isLoading = true;
    const { rerender } = renderStatus();
    expect(screen.getByTestId("provider-catalog-loading")).toHaveTextContent(
      "Loading catalog status…"
    );

    mockStatusQuery.isLoading = false;
    mockStatusQuery.data = { sources: [] };
    rerender(
      <UIProvider reducedMotion="always">
        <ProviderModelCatalogStatus providerId="codex" testId="provider-catalog" />
      </UIProvider>
    );
    expect(screen.getByTestId("provider-catalog-empty")).toHaveTextContent(
      "No catalog sources reporting yet."
    );
  });
});
