import type { Meta, StoryObj } from "@storybook/react-vite";
import { LayoutDashboard, ListChecks } from "lucide-react";

import { Button } from "../../button";
import { ListingToolbar } from "../listing-toolbar";
import { RouteNav } from "../route-nav";
import { Topbar, TopbarOverflowIcon, TopbarSlotProvider, useTopbarSlot } from "../topbar";

const meta: Meta<typeof Topbar> = {
  title: "components/custom/Topbar",
  component: Topbar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Unified window head (44px): traffic lights · quiet glyph + title (or window-local drill-in trail) · optional peer RouteNav after identity · status + ≤2 actions. Routes publish identity/nav/tools via `useTopbarSlot`.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-full border border-line bg-background">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * T1 · Root identity — quiet glyph + title.
 */
export const RootIdentity: Story = {
  args: {},
  render: () => (
    <TopbarSlotProvider>
      <Topbar glyph={<LayoutDashboard />} title="Dashboard" />
    </TopbarSlotProvider>
  ),
};

function DetailActionsSetup() {
  useTopbarSlot({
    onBack: () => undefined,
    crumbs: [{ id: "loops", label: "Loops", onSelect: () => undefined }],
    crumb: "software-delivery",
    actions: (
      <div className="flex items-center gap-2">
        <Button size="sm" variant="ghost">
          Edit
        </Button>
        <Button size="sm">Run loop</Button>
      </div>
    ),
    overflow: (
      <Button aria-label="More actions" size="sm" variant="ghost">
        <TopbarOverflowIcon className="size-3" />
      </Button>
    ),
  });
  return null;
}

/**
 * T2 · Drill-in — back + parent crumbs + leaf + trailing actions.
 */
export const DrillInActions: Story = {
  args: {},
  render: () => (
    <TopbarSlotProvider>
      <DetailActionsSetup />
      <Topbar title="Loops" />
    </TopbarSlotProvider>
  ),
};

function FullCompositionSetup() {
  useTopbarSlot({
    glyph: <ListChecks />,
    count: 12,
    actions: <Button size="sm">New task</Button>,
    nav: (
      <RouteNav aria-label="Tasks views">
        <RouteNav.Link aria-current="page" href="#list">
          List
        </RouteNav.Link>
        <RouteNav.Link href="#kanban">Kanban</RouteNav.Link>
        <RouteNav.Link href="#inbox">
          Inbox <RouteNav.Count>2</RouteNav.Count>
        </RouteNav.Link>
      </RouteNav>
    ),
    toolbar: (
      <ListingToolbar className="w-full">
        <ListingToolbar.Leading>
          <span className="text-xs text-subtle">filters</span>
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <span className="text-xs text-subtle">Rows | Cards</span>
        </ListingToolbar.Trailing>
      </ListingToolbar>
    ),
  });
  return null;
}

/**
 * T3 · Catalog root — peer RouteNav after identity; tools in the frame strip.
 */
export const CatalogWithTools: Story = {
  args: {},
  render: () => (
    <TopbarSlotProvider>
      <FullCompositionSetup />
      <Topbar title="Tasks" />
      <div
        data-slot="os-window-toolbar"
        className="flex min-h-[38px] items-center border-b border-line px-3 text-xs text-subtle"
      >
        filters · Rows | Cards
      </div>
    </TopbarSlotProvider>
  ),
};

function TasksDrillInSetup() {
  useTopbarSlot({
    onBack: () => undefined,
    crumbs: [
      { id: "tasks", label: "Tasks", onSelect: () => undefined },
      { id: "task", label: "Refresh marketplace index", onSelect: () => undefined },
    ],
    crumb: "Run #128",
    status: <span className="text-xs text-subtle">running</span>,
    actions: (
      <Button size="sm" variant="ghost">
        Cancel
      </Button>
    ),
  });
  return null;
}

/**
 * T4 · Tasks two-level drill-in (list → detail → run).
 */
export const TasksDrillIn: Story = {
  args: {},
  render: () => (
    <TopbarSlotProvider>
      <TasksDrillInSetup />
      <Topbar title="Tasks" />
    </TopbarSlotProvider>
  ),
};

function MarketplaceDrillInSetup() {
  useTopbarSlot({
    onBack: () => undefined,
    crumbs: [{ id: "marketplace", label: "Marketplace", onSelect: () => undefined }],
    crumb: "Claude Code",
    status: <span className="text-xs text-subtle">Not installed</span>,
    actions: <Button size="sm">Install</Button>,
  });
  return null;
}

/**
 * T5 · Marketplace in-place entry under Marketplace / entry.
 */
export const MarketplaceDrillIn: Story = {
  args: {},
  render: () => (
    <TopbarSlotProvider>
      <MarketplaceDrillInSetup />
      <Topbar title="Marketplace" />
    </TopbarSlotProvider>
  ),
};

function SessionDocumentSetup() {
  useTopbarSlot({
    status: <span className="text-xs text-subtle">2m 14s</span>,
    actions: <span className="text-xs text-subtle">claude-opus · Claude Code</span>,
  });
  return null;
}

/**
 * T6 · Document/session self-title — state mark replaces glyph; no crumbs.
 */
export const SessionDocument: Story = {
  args: {},
  render: () => (
    <TopbarSlotProvider>
      <SessionDocumentSetup />
      <Topbar
        leading={
          <span
            aria-hidden
            className="size-2 shrink-0 rounded-full bg-success"
            data-slot="topbar-state-dot"
          />
        }
        title="Prune stale sessions"
      />
    </TopbarSlotProvider>
  ),
};
