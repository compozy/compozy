import type { Meta, StoryObj } from "@storybook/react-vite";
import { ListFilter } from "lucide-react";
import { useState } from "react";

import { Button } from "../../button";
import { ListingToolbar, type ListingViewMode } from "../listing-toolbar";

function ListingToolbarDemo({
  initialView = "rows" as ListingViewMode,
  withFilters = true,
  withViewToggle = true,
  collapsedSearch = false,
}: {
  initialView?: ListingViewMode;
  withFilters?: boolean;
  withViewToggle?: boolean;
  collapsedSearch?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [view, setView] = useState<ListingViewMode>(initialView);

  return (
    <div
      className={
        collapsedSearch
          ? "w-[300px] rounded-lg border border-line bg-canvas p-4"
          : "w-[720px] rounded-lg border border-line bg-canvas p-4"
      }
    >
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search
            collapsed={collapsedSearch}
            onChange={setSearch}
            placeholder="Search skills"
            value={search}
          />
          {withFilters ? (
            <ListingToolbar.Filters>
              <Button size="segment" type="button" variant="quiet">
                <ListFilter aria-hidden="true" />
                Filter
              </Button>
            </ListingToolbar.Filters>
          ) : null}
        </ListingToolbar.Leading>
        {withViewToggle ? (
          <ListingToolbar.Trailing>
            <ListingToolbar.ViewToggle onChange={setView} value={view} />
          </ListingToolbar.Trailing>
        ) : null}
      </ListingToolbar>
    </div>
  );
}

const meta: Meta<typeof ListingToolbarDemo> = {
  title: "components/custom/ListingToolbar",
  component: ListingToolbarDemo,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Compound inventory toolbar. Compose `Leading` (Search + Filters) and optional `Trailing` (ViewToggle). Omit slots you do not need — no boolean props on the root.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => <ListingToolbarDemo />,
};

export const CardsActive: Story = {
  render: () => <ListingToolbarDemo initialView="cards" />,
};

export const SearchOnly: Story = {
  render: () => <ListingToolbarDemo withFilters={false} withViewToggle={false} />,
};

/** A strip too narrow for a usable field: search folds to a toggle that opens it on demand. */
export const CollapsedSearch: Story = {
  render: () => <ListingToolbarDemo collapsedSearch withViewToggle={false} />,
};
