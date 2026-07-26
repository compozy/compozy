import { Repeat2 } from "lucide-react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Avatar, AvatarFallback } from "../../avatar";
import { Button } from "../../button";
import { ListingRow } from "../listing-row";
import { Pill } from "../pill";

const meta: Meta<typeof ListingRow> = {
  title: "components/custom/ListingRow",
  component: ListingRow,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Canonical main-pane inventory row. Grid: 34px icon well · main · trail. Hover uses `--row-hover`; selected uses `--row-selected` via `data-selected`. The link region spans icon + main; trail actions stay siblings.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[720px] overflow-hidden rounded-lg border border-line bg-canvas-soft">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function FullRow({ selected = false }: { selected?: boolean }) {
  return (
    <ListingRow selected={selected}>
      <ListingRow.Link href="#alpha" aria-label="Open alpha-delivery">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>alpha-delivery</ListingRow.Title>
            <Pill size="xs" tone="neutral">
              Workspace
            </Pill>
            <ListingRow.Slug>v1.2.0</ListingRow.Slug>
          </ListingRow.Name>
          <ListingRow.Description>
            Ships a release candidate through the gate.
          </ListingRow.Description>
          <ListingRow.Meta>
            <span className="font-mono text-[10px] text-subtle">3 inputs</span>
            <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
            <span>iteration cap 8</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Pill mono size="sm" tone="neutral">
          delivery
        </Pill>
        <ListingRow.Stat>
          <ListingRow.Stat.Value>92%</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>12 runs · 30d</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <Button size="sm" type="button" variant="outline">
          Run
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  );
}

export const Default: Story = {
  render: () => <FullRow />,
};

export const NoDescription: Story = {
  render: () => (
    <ListingRow>
      <ListingRow.Link href="#beta" aria-label="Open beta-watch">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>beta-watch</ListingRow.Title>
            <ListingRow.Slug>v0.9.1</ListingRow.Slug>
          </ListingRow.Name>
          <ListingRow.Meta>
            <span className="font-mono text-[10px] text-subtle">1 input</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Button size="sm" type="button" variant="outline">
          Run
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  ),
};

export const NoMeta: Story = {
  render: () => (
    <ListingRow>
      <ListingRow.Link href="#gamma" aria-label="Open gamma">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>gamma</ListingRow.Title>
          </ListingRow.Name>
          <ListingRow.Description>One truncated description line only.</ListingRow.Description>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Button size="sm" type="button" variant="outline">
          Open
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  ),
};

export const MonoName: Story = {
  render: () => (
    <ListingRow>
      <ListingRow.Link href="#vault-key" aria-label="Open vault.key">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name mono>
            <ListingRow.Title>sessions/operator.token</ListingRow.Title>
            <Pill size="xs" tone="neutral">
              secret
            </Pill>
          </ListingRow.Name>
          <ListingRow.Meta>
            <span>updated 2h ago</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Pill mono size="sm" tone="neutral">
          sessions
        </Pill>
      </ListingRow.Trail>
    </ListingRow>
  ),
};

export const WithTags: Story = {
  render: () => (
    <ListingRow>
      <ListingRow.Link href="#tagged" aria-label="Open tagged-item">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>tagged-item</ListingRow.Title>
            <Pill size="xs" tone="neutral">
              Read-only
            </Pill>
            <Pill size="xs" tone="neutral">
              ∞ cap
            </Pill>
            <ListingRow.Slug>v2.0.0</ListingRow.Slug>
          </ListingRow.Name>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Pill mono size="sm" tone="neutral">
          watch
        </Pill>
      </ListingRow.Trail>
    </ListingRow>
  ),
};

export const StatBlock: Story = {
  render: () => (
    <ListingRow>
      <ListingRow.Link href="#stats" aria-label="Open stats-row">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>stats-row</ListingRow.Title>
          </ListingRow.Name>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <ListingRow.Stat>
          <ListingRow.Stat.Value>87%</ListingRow.Stat.Value>
          <ListingRow.Stat.Label>24 runs · 30d</ListingRow.Stat.Label>
        </ListingRow.Stat>
        <Button size="sm" type="button" variant="outline">
          Run
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  ),
};

export const Selected: Story = {
  render: () => (
    <>
      <FullRow />
      <FullRow selected />
      <FullRow />
    </>
  ),
};

export const AvatarInWell: Story = {
  render: () => (
    <ListingRow selected>
      <ListingRow.Link href="#peer" aria-label="Open @operator">
        <ListingRow.Icon>
          <Avatar shape="circle" size="sm">
            <AvatarFallback>OP</AvatarFallback>
          </Avatar>
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>@operator</ListingRow.Title>
            <Pill size="xs" tone="neutral">
              peer
            </Pill>
          </ListingRow.Name>
          <ListingRow.Description>Last message preview line.</ListingRow.Description>
          <ListingRow.Meta>
            <span>2h ago</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <span className="text-[11px] text-faint">2h</span>
      </ListingRow.Trail>
    </ListingRow>
  ),
};

export const TrailAction: Story = {
  render: () => (
    <ListingRow>
      <ListingRow.Link href="#action" aria-label="Open action-row">
        <ListingRow.Icon>
          <Repeat2 aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>action-row</ListingRow.Title>
          </ListingRow.Name>
          <ListingRow.Description>
            Primary action lives in the trail, outside the link.
          </ListingRow.Description>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Button size="sm" type="button">
          Run
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  ),
};
