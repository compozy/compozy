import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Button, Filters, FiltersWithSearch, type FilterFieldsConfig } from "@agh/ui";

const FIELDS: FilterFieldsConfig<boolean> = [
  {
    key: "has_work",
    label: "Has work",
    type: "toggle",
  },
];

describe("Filters public surface", () => {
  it("Should resolve Filters from @agh/ui and render an accessible add-filter control", () => {
    const onChange = vi.fn();

    render(
      <Filters<boolean>
        allowMultiple={false}
        fields={FIELDS}
        filters={[]}
        onChange={onChange}
        size="sm"
        trigger={
          <Button size="sm" variant="ghost" aria-label="Add filter">
            Filter
          </Button>
        }
      />
    );

    expect(screen.getByRole("button", { name: "Add filter" })).toBeInTheDocument();
  });

  it("Should expose each custom comparison operator exactly once", async () => {
    const user = userEvent.setup();

    render(
      <Filters<string>
        allowMultiple={false}
        fields={[
          {
            key: "created_at",
            label: "Created at",
            type: "custom",
            customRenderer: () => <span>Value</span>,
          },
        ]}
        filters={[{ id: "created-at", field: "created_at", operator: "is", values: [] }]}
        onChange={vi.fn()}
      />
    );

    await user.click(screen.getByRole("button", { name: "is" }));
    await screen.findByRole("menuitem", { name: "before" });

    expect(screen.getAllByRole("menuitem").map(item => item.textContent)).toEqual([
      "is",
      "after",
      "before",
      "between",
      "is empty",
      "is not empty",
    ]);
  });

  it("Should resolve FiltersWithSearch from @agh/ui as a distinct searchable variant", () => {
    const onChange = vi.fn();

    render(
      <FiltersWithSearch<boolean>
        allowMultiple={false}
        fields={FIELDS}
        filters={[]}
        onChange={onChange}
        size="sm"
        trigger={
          <Button size="sm" variant="ghost" aria-label="Add filter with search">
            Filter
          </Button>
        }
      />
    );

    expect(screen.getByRole("button", { name: "Add filter with search" })).toBeInTheDocument();
    expect(FiltersWithSearch).not.toBe(Filters);
  });
});
