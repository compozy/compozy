import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn } from "storybook/test";

import type { OsWindow } from "../../lib/os-types";
import { projectTab } from "../../lib/tab-projection";
import type { LayoutNode } from "../../lib/window-manager-types";
import { DesktopLayoutThumbnail } from "../desktop-layout-thumbnail";

import {
  DesktopsOverview,
  type DesktopOverviewItem,
  type DesktopsOverviewProps,
} from "../desktops-overview";
import { DesktopShell } from "./_desktop";

const WORK_AREA = { x: 0, y: 0, w: 1380, h: 848 };
const NO_GAPS = { inner: 0, top: 0, right: 0, bottom: 0, left: 0 };

function storyWindow(id: string, app: OsWindow["app"]): OsWindow {
  return {
    id,
    app,
    instanceKey: null,
    route: { pathname: `/${app}`, search: {} },
    navStack: [],
    desktopId: "story",
    tabId: "story-tab",
    nodeId: id,
    pinned: false,
    inSplit: true,
    tabActive: true,
    shown: true,
  };
}

function leaf(windowId: string): LayoutNode {
  return { id: windowId, kind: "leaf", windowId };
}

function split(
  id: string,
  axis: "horizontal" | "vertical",
  children: LayoutNode[],
  weights: number[]
): LayoutNode {
  return { id, kind: "split", axis, children, weights };
}

/** A thumbnail over the real projection of the desktop's active tab. */
function thumbnail(root: LayoutNode, apps: Record<string, OsWindow["app"]>) {
  const projection = projectTab({
    tab: { id: "story-tab", pinned: false, root },
    workArea: WORK_AREA,
    gaps: NO_GAPS,
  });
  const windows = Object.entries(apps).map(([id, app]) => storyWindow(id, app));
  return <DesktopLayoutThumbnail projection={projection} windows={windows} />;
}

const DESKTOPS: DesktopOverviewItem[] = [
  {
    id: "control",
    name: "Main",
    aspectRatio: WORK_AREA.w / WORK_AREA.h,
    switchShortcut: "⌃1",
    thumbnail: thumbnail(
      split(
        "main",
        "horizontal",
        [leaf("s1"), split("side", "vertical", [leaf("tasks"), leaf("term")], [0.56, 0.44])],
        [0.55, 0.45]
      ),
      { s1: "session", tasks: "tasks", term: "terminal" }
    ),
    windows: [
      { id: "s1", title: "Session" },
      { id: "s2", title: "Session" },
      { id: "tasks", title: "Tasks" },
      { id: "term", title: "Terminal" },
    ],
  },
  {
    id: "build",
    name: "Review",
    aspectRatio: WORK_AREA.w / WORK_AREA.h,
    switchShortcut: "⌃2",
    needsYou: true,
    thumbnail: thumbnail(
      split("review", "horizontal", [leaf("agents"), leaf("loops")], [0.62, 0.38]),
      { agents: "agents", loops: "loops" }
    ),
    windows: [
      { id: "agents", title: "Agents" },
      { id: "loops", title: "Loops" },
      { id: "t1", title: "Terminal" },
    ],
  },
  {
    id: "research",
    name: "Research",
    aspectRatio: WORK_AREA.w / WORK_AREA.h,
    switchShortcut: "⌃3",
    thumbnail: thumbnail(leaf("marketplace"), { marketplace: "marketplace" }),
    windows: [
      { id: "marketplace", title: "Marketplace" },
      { id: "vault", title: "Vault" },
    ],
  },
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
  onMoveWindow: fn<DesktopsOverviewProps["onMoveWindow"]>(),
};

const meta: Meta<typeof DesktopsOverview> = {
  title: "systems/os/components/DesktopsOverview",
  component: DesktopsOverview,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "On-demand desktop management over the desk (shell-rail v2 `.ov`): live layout thumbnails, keycaps, needs-you dots, and New desktop; each card's menu renames, reorders, moves a window, or deletes with transfer. Arrows move across cards, Enter switches, Esc closes.",
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
