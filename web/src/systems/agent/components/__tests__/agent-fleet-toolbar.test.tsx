import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

import { AgentFleetToolbar } from "../agent-fleet-toolbar";

vi.mock("@agh/ui", async importOriginal => {
  const actual = await importOriginal<typeof import("@agh/ui")>();
  return {
    ...actual,
    FiltersWithSearch: ({
      onChange,
      "data-testid": testId,
    }: {
      onChange: (next: { id: string; field: string; operator: string; values: string[] }[]) => void;
      "data-testid"?: string;
    }) => (
      <div data-testid={testId}>
        <button
          data-testid="mock-set-category"
          onClick={() =>
            onChange([
              {
                id: "agent-fleet-filter-category",
                field: "category",
                operator: "is",
                values: ["Ops"],
              },
            ])
          }
          type="button"
        >
          Set category
        </button>
        <button
          data-testid="mock-set-status"
          onClick={() =>
            onChange([
              {
                id: "agent-fleet-filter-status",
                field: "status",
                operator: "is",
                values: ["active"],
              },
            ])
          }
          type="button"
        >
          Set status
        </button>
      </div>
    ),
  };
});

describe("AgentFleetToolbar", () => {
  it("Should forward filter chip changes to category and status handlers", async () => {
    const user = userEvent.setup();
    const onFiltersChange = vi.fn();
    render(
      <AgentFleetToolbar
        categoryOptions={["Ops"]}
        draftQuery=""
        onDraftQueryChange={vi.fn()}
        onFiltersChange={onFiltersChange}
        onViewChange={vi.fn()}
        search={{}}
        searchInputRef={createRef<HTMLInputElement>()}
        showFacets
        view="rows"
      />
    );

    await user.click(screen.getByTestId("mock-set-category"));
    expect(onFiltersChange).toHaveBeenCalledWith({ category: "Ops", status: undefined });

    await user.click(screen.getByTestId("mock-set-status"));
    expect(onFiltersChange).toHaveBeenLastCalledWith({ category: undefined, status: "active" });
  });

  it("Should expose the view toggle independently of facets", async () => {
    const user = userEvent.setup();
    const onViewChange = vi.fn();
    const { rerender } = render(
      <AgentFleetToolbar
        categoryOptions={["Ops"]}
        draftQuery=""
        onDraftQueryChange={vi.fn()}
        onFiltersChange={vi.fn()}
        onViewChange={onViewChange}
        search={{}}
        searchInputRef={createRef<HTMLInputElement>()}
        showFacets
        showViewToggle
        view="rows"
      />
    );

    expect(screen.getByTestId("listing-view-toggle")).toBeInTheDocument();
    await user.click(screen.getByTestId("listing-view-cards"));
    expect(onViewChange).toHaveBeenCalledWith("cards");

    rerender(
      <AgentFleetToolbar
        categoryOptions={["Ops"]}
        draftQuery=""
        onDraftQueryChange={vi.fn()}
        onFiltersChange={vi.fn()}
        onViewChange={onViewChange}
        search={{}}
        searchInputRef={createRef<HTMLInputElement>()}
        showFacets={false}
        showViewToggle
        view="rows"
      />
    );
    expect(screen.getByTestId("listing-view-toggle")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-fleet-filters")).not.toBeInTheDocument();

    rerender(
      <AgentFleetToolbar
        categoryOptions={["Ops"]}
        draftQuery=""
        onDraftQueryChange={vi.fn()}
        onFiltersChange={vi.fn()}
        onViewChange={onViewChange}
        search={{}}
        searchInputRef={createRef<HTMLInputElement>()}
        showFacets={false}
        showViewToggle={false}
        view="rows"
      />
    );
    expect(screen.queryByTestId("listing-view-toggle")).not.toBeInTheDocument();
  });
});
