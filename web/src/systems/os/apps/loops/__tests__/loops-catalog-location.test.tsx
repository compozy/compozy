// Suite: Loops catalog route composition
// Invariant: the catalog's first read is waiting, not empty and not failed. A pending read holds
// the catalog's own frame inside the page its rows will fill, with the toolbar already published;
// only a failure with nothing to keep shows the retryable error.
// Boundary IN: LoopsCatalogLocation query-state routing and retry wiring.
// Boundary OUT: the catalog's rows and the query state itself, owned by their component and model suites.

import { createElement, type ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useTopbarSlot } from "@compozy/ui";

const { useLoopsCatalogMock } = vi.hoisted(() => ({ useLoopsCatalogMock: vi.fn() }));

vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => vi.fn(),
  Link: ({ to, children, ...props }: { to: string; children?: ReactNode }) =>
    createElement("a", { href: to, ...props }, children),
}));

vi.mock("@compozy/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@compozy/ui")>();
  return { ...actual, useTopbarSlot: vi.fn() };
});

vi.mock("../use-loops-catalog", () => ({ useLoopsCatalog: useLoopsCatalogMock }));

const { LoopsCatalogLocation } = await import("../loops-catalog-location");

function catalogPage(overrides: Record<string, unknown> = {}) {
  return {
    ready: true,
    allProjects: false,
    aggregate: undefined,
    categoryOptions: [],
    clearFilters: vi.fn(),
    definitionsNote: null,
    entries: [],
    facets: undefined,
    filter: { kind: "all", category: null, status: null },
    handleRun: vi.fn(),
    hasActiveFilters: false,
    projectFilter: { visible: false, values: [], field: null, setValues: vi.fn(), clear: vi.fn() },
    searchQuery: "",
    setFilters: vi.fn(),
    setSearchQuery: vi.fn(),
    setView: vi.fn(),
    total: 0,
    view: "rows",
    workspaceLabel: "launch-hq",
    catalogQuery: {
      isPending: false,
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: vi.fn(),
      retry: vi.fn(),
    },
    ...overrides,
  };
}

describe("LoopsCatalogLocation", () => {
  beforeEach(() => {
    vi.mocked(useTopbarSlot).mockClear();
    useLoopsCatalogMock.mockReturnValue(catalogPage());
  });

  it("Should hold the catalog's frame, not an empty state, while the first read is pending", () => {
    useLoopsCatalogMock.mockReturnValue(
      catalogPage({
        catalogQuery: { ...catalogPage().catalogQuery, isPending: true },
      })
    );

    const { container } = render(<LoopsCatalogLocation search={{}} />);

    const pending = screen.getByTestId("loops-loading");
    expect(container.querySelector('[data-slot="listing-page"]')).toBe(pending);
    expect(within(pending).getByRole("status")).toHaveTextContent("Loading Loops");
    expect(screen.queryByTestId("loop-catalog-empty")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loops-error")).not.toBeInTheDocument();
    // The toolbar does not wait for rows: it is published with the placeholder, so nothing moves.
    const slot = vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0];
    expect(slot?.toolbar).toBeDefined();
    expect(slot?.actions).toBeDefined();
  });

  it("Should stay blank, with no toolbar, until the window's data scope settles", () => {
    useLoopsCatalogMock.mockReturnValue(catalogPage({ ready: false }));

    render(<LoopsCatalogLocation search={{}} />);

    expect(screen.getByTestId("loops-no-workspace")).toHaveAttribute("aria-busy", "true");
    expect(screen.queryByTestId("loops-loading")).not.toBeInTheDocument();
    expect(vi.mocked(useTopbarSlot).mock.calls.at(-1)?.[0]?.toolbar).toBeUndefined();
  });

  it("Should offer a retry when the read failed and there is nothing to keep", async () => {
    const retry = vi.fn();
    useLoopsCatalogMock.mockReturnValue(
      catalogPage({
        catalogQuery: {
          ...catalogPage().catalogQuery,
          error: new Error("Transport unavailable"),
          retry,
        },
      })
    );

    render(<LoopsCatalogLocation search={{}} />);

    expect(screen.getByTestId("loops-error")).toHaveTextContent("Transport unavailable");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });
});
