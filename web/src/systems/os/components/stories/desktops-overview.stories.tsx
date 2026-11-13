import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import type { OsWindow } from "../../lib/os-types";
import type { LayoutNode } from "../../lib/window-manager-types";
import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopOverviewTab,
  type DesktopsOverviewProps,
} from "../desktops-overview";
import { DesktopShell } from "./_desktop";
import { STORY_WINDOWS as W, storySessionOf } from "./_tab-fixtures";

function leaf(win: OsWindow): LayoutNode {
  return { id: win.nodeId, kind: "leaf", windowId: win.id };
}

function split(
  id: string,
  axis: "horizontal" | "vertical",
  children: LayoutNode[],
  weights: number[]
): LayoutNode {
  return { id, kind: "split", axis, children, weights };
}

function tab(id: string, ...members: OsWindow[]): DesktopOverviewTab {
  return { id, leadWindowId: members[0]!.id, members };
}

const CONTROL: DesktopOverviewItem = {
  id: "control",
  name: "Main",
  switchShortcut: "⌃1",
  tabs: [tab("t-main", W.auth, W.tasks, W.terminal), tab("t-dash", W.dash)],
  activeTabId: "t-main",
  activeRoot: split(
    "main",
    "horizontal",
    [leaf(W.auth), split("side", "vertical", [leaf(W.tasks), leaf(W.terminal)], [0.56, 0.44])],
    [0.55, 0.45]
  ),
};

const REVIEW: DesktopOverviewItem = {
  id: "review",
  name: "Review",
  switchShortcut: "⌃2",
  needsYou: true,
  tabs: [tab("t-review", W.review, W.agents), tab("t-files", W.files)],
  activeTabId: "t-review",
  activeRoot: split("review", "horizontal", [leaf(W.review), leaf(W.agents)], [0.62, 0.38]),
};

const RESEARCH: DesktopOverviewItem = {
  id: "research",
  name: "Research",
  switchShortcut: "⌃3",
  tabs: [tab("t-docs", W.docs)],
  activeTabId: "t-docs",
  activeRoot: leaf(W.docs),
};

const DESKTOPS: DesktopOverviewItem[] = [CONTROL, REVIEW, RESEARCH];

/** A desktop with more tabs than its card lists: the rest count as "+N more". */
const FULL: DesktopOverviewItem[] = [
  {
    ...CONTROL,
    tabs: [
      ...CONTROL.tabs,
      tab("t-tabs", W.tabs),
      tab("t-agents", W.agents),
      tab("t-files2", W.files),
      tab("t-docs2", W.docs),
    ],
  },
  REVIEW,
  RESEARCH,
];

/** Mounts the overview in a desk-sized overlay host, as `DesktopManagerSurfaces` does. */
function InDesk(props: DesktopsOverviewProps) {
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  return (
    <DesktopShell>
      <div
        ref={setHost}
        className="contain-paint pointer-events-none absolute inset-0 z-40 [&:not(:empty)]:pointer-events-auto"
      />
      <DesktopsOverview {...props} container={host} />
    </DesktopShell>
  );
}

const ACTIONS = {
  onOpenChange: fn<DesktopsOverviewProps["onOpenChange"]>(),
  onCreateDesktop: fn<DesktopsOverviewProps["onCreateDesktop"]>(),
  onSwitchDesktop: fn<DesktopsOverviewProps["onSwitchDesktop"]>(),
  onRenameDesktop: fn<DesktopsOverviewProps["onRenameDesktop"]>(),
  onReorderDesktop: fn<DesktopsOverviewProps["onReorderDesktop"]>(),
  onDeleteDesktop: fn<DesktopsOverviewProps["onDeleteDesktop"]>(),
  onMoveTab: fn<DesktopsOverviewProps["onMoveTab"]>(),
  onChooseTab: fn<DesktopsOverviewProps["onChooseTab"]>(),
  onTabPointerDown: fn<NonNullable<DesktopsOverviewProps["onTabPointerDown"]>>(),
  sessionOf: storySessionOf,
};

const meta: Meta<typeof DesktopsOverview> = {
  title: "systems/os/components/DesktopsOverview",
  component: DesktopsOverview,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "On-demand desktop management over the desk: one card per desktop with its layout thumbnail, tab count and tabs, rename, and the in-place confirm close that asks where the tabs go. Cards drag to reorder and take a dragged tab; the card's context menu keeps Rename, Move left / Move right and Move a tab. Choosing a tab jumps to it. Arrows move across cards, Enter switches, Esc closes.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Resolved workspace state with one current desktop and one focus desktop. */
export const Ready: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: DESKTOPS, activeDesktopId: "control" },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** The last desktop has no close control (US-014.EC-2). */
export const OneDesktop: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: [CONTROL], activeDesktopId: "control" },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** A card lists its first tabs and counts the rest (US-014.EC-3). */
export const ManyTabs: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: FULL, activeDesktopId: "control" },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** A dragged tab is over the Review card: releasing moves it there. */
export const TabDropTarget: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: DESKTOPS, activeDesktopId: "control" },
    dropTargetDesktopId: "review",
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** Closing a desktop with tabs asks where they go before the check confirms. */
export const CloseWithTabs: Story = {
  tags: ["play-fn"],
  args: {
    open: true,
    state: { status: "ready", desktops: DESKTOPS, activeDesktopId: "control" },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
  play: async ({ args, canvasElement }) => {
    const page = within(canvasElement.ownerDocument.body);
    await userEvent.click(await page.findByRole("button", { name: "Close Review" }));
    const confirm = page.getByRole("button", { name: "Confirm: Close Review" });
    await expect(confirm).toBeDisabled();
    await userEvent.selectOptions(page.getByRole("combobox"), "Main");
    await userEvent.click(confirm);
    await expect(args.onDeleteDesktop).toHaveBeenCalledWith("review", "control");
  },
};

/** Initial snapshot acquisition keeps the final card geometry stable. */
export const Loading: Story = {
  args: { open: true, state: { status: "loading" }, ...ACTIONS },
  render: args => <InDesk {...args} />,
};

/** Transport failure exposes a single recovery action without fabricated desktop data. */
export const Error: Story = {
  args: {
    open: true,
    state: { status: "error", message: "The workspace desktop snapshot is unavailable." },
    onRetry: fn(),
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** Revision conflict asks the shell to reload before accepting more mutations. */
export const Conflict: Story = {
  args: {
    open: true,
    state: {
      status: "conflict",
      message: "Another client changed the desktop order after this view opened.",
    },
    onResolveConflict: fn(),
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};

/** A real empty workspace offers desktop creation as its only next action. */
export const Empty: Story = {
  args: {
    open: true,
    state: { status: "ready", desktops: [], activeDesktopId: null },
    ...ACTIONS,
  },
  render: args => <InDesk {...args} />,
};
