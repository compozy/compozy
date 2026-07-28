import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Button } from "../../button";
import { ListingToolbar, type ListingViewMode } from "../listing-toolbar";

function ListingToolbarDemo({
  initialView = "rows" as ListingViewMode,
  withFilters = true,
  withViewToggle = true,
}: {
  initialView?: ListingViewMode;
  withFilters?: boolean;
  withViewToggle?: boolean;
}) {
  const [search, setSearch] = useState("");
  const [view, setView] = useState<ListingViewMode>(initialView);

  return (
    <div className="w-[720px] rounded-lg border border-line bg-canvas p-4">
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search onChange={setSearch} placeholder="Search skills" value={search} />
          {withFilters ? (
            <ListingToolbar.Filters>
              <Button size="sm" type="button" variant="ghost">
                + Filter
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
