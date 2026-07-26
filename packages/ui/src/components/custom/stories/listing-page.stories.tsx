import type { Meta, StoryObj } from "@storybook/react-vite";
import { Repeat2 } from "lucide-react";
import { useState } from "react";

import { Button } from "../../button";
import { ListingPage } from "../listing-page";
import { ListingRow } from "../listing-row";
import { ListingToolbar, type ListingViewMode } from "../listing-toolbar";
import { Pill } from "../pill";

const ROWS = [
  {
    name: "reviews-watch",
    desc: "React to inbound review requests as they arrive.",
    kind: "watch",
  },
  {
    name: "software-delivery",
    desc: "Ship the requested change end to end and prove it.",
    kind: "delivery",
  },
];

function ListingPageDemo() {
  const [search, setSearch] = useState("");
  const [view, setView] = useState<ListingViewMode>("rows");

  return (
    <ListingPage>
      <p className="mb-3 text-xs text-subtle">
        Reusable, guardrailed cycles that pursue a goal until it is verified · launch-hq
      </p>
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search onChange={setSearch} placeholder="Search loops" value={search} />
          <ListingToolbar.Filters>
            <Button size="sm" type="button" variant="ghost">
              + Filter
            </Button>
          </ListingToolbar.Filters>
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={setView} value={view} />
        </ListingToolbar.Trailing>
      </ListingToolbar>
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {ROWS.map(row => (
          <ListingRow key={row.name}>
            <ListingRow.Link render={<a href="#loop" />}>
              <ListingRow.Icon>
                <Repeat2 aria-hidden="true" className="size-4" />
              </ListingRow.Icon>
              <ListingRow.Main>
                <ListingRow.Name>
                  <ListingRow.Title>{row.name}</ListingRow.Title>
                </ListingRow.Name>
                <ListingRow.Description>{row.desc}</ListingRow.Description>
              </ListingRow.Main>
            </ListingRow.Link>
            <ListingRow.Trail>
              <Pill mono size="sm" tone="neutral">
                {row.kind}
              </Pill>
            </ListingRow.Trail>
          </ListingRow>
        ))}
      </div>
    </ListingPage>
  );
}

const meta: Meta<typeof ListingPageDemo> = {
  title: "components/custom/ListingPage",
  component: ListingPageDemo,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Canonical inventory listing shell: scroll area + width-capped, centered content container. Route identity (glyph · title · count) lives in the OS window head; this shell owns the body list + optional toolbar.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => <ListingPageDemo />,
};
