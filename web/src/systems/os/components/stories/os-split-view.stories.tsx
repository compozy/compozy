import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { getOsAppDescriptor } from "../../lib/app-catalog";
import type { OsWindow } from "../../lib/os-types";
import type { CollapsedSplit, LayoutNode } from "../../lib/window-manager-types";
import { OsSplitView } from "../os-split-view";
import { STORY_WINDOWS as W, storySessionOf } from "./_tab-fixtures";

const meta: Meta = {
  title: "systems/os/components/OsSplitView",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "A tab's content: its split tree laid out from the snapshot's weights, with hairline seams between members. A split that cannot fit — or every split in the compact viewport — shows one member at a time behind a switcher, leaving the weights untouched.",
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const WINDOWS: Record<string, OsWindow> = Object.fromEntries(
  [W.tabs, W.terminal, W.files].map(win => [win.id, { ...win, inSplit: true, tabActive: true }])
);

function leaf(windowId: string): LayoutNode {
  return { id: `leaf:${windowId}`, kind: "leaf", windowId };
}

const TWO_UP: LayoutNode = {
  id: "split:two-up",
  kind: "split",
  axis: "horizontal",
  weights: [0.62, 0.38],
  children: [leaf("w-tabs"), leaf("w-terminal")],
};

const MAIN_STACK: LayoutNode = {
  id: "split:main",
  kind: "split",
  axis: "horizontal",
  weights: [0.55, 0.45],
  children: [
    leaf("w-tabs"),
    {
      id: "split:stack",
      kind: "split",
      axis: "vertical",
      weights: [0.5, 0.5],
      children: [leaf("w-terminal"), leaf("w-files")],
    },
  ],
};

function Pane({ windowId, shown = true }: { windowId: string; shown?: boolean }) {
  const win = WINDOWS[windowId]!;
  return (
    <section
      aria-label={`${getOsAppDescriptor(win.app).title} window`}
      className="flex min-h-0 min-w-0 flex-1 items-center justify-center bg-canvas text-body text-subtle"
      hidden={!shown}
    >
      {storySessionOf(win)?.name ?? getOsAppDescriptor(win.app).title}
    </section>
  );
}

function SplitStory({ root, collapsed = [] }: { root: LayoutNode; collapsed?: CollapsedSplit[] }) {
  const hidden = new Set(
    collapsed.flatMap(entry => entry.windowIds.filter(id => id !== entry.shownWindowId))
  );
  return (
    <div className="flex h-[560px] w-full">
      <OsSplitView
        collapsed={collapsed}
        onShowMember={fn()}
        renderWindow={windowId => <Pane shown={!hidden.has(windowId)} windowId={windowId} />}
        root={root}
        sessionOf={storySessionOf}
        windows={WINDOWS}
      />
    </div>
  );
}

/** Two windows side by side at the snapshot's 62/38 weights. */
export const TwoUp: Story = { render: () => <SplitStory root={TWO_UP} /> };

/** A main window beside a vertical stack: nested splits keep their own axis and weights. */
export const MainAndStack: Story = { render: () => <SplitStory root={MAIN_STACK} /> };

/** The stack cannot fit its members: it shows one at a time behind the member switcher. */
export const CollapsedStack: Story = {
  render: () => (
    <SplitStory
      collapsed={[
        {
          splitId: "split:stack",
          windowIds: ["w-terminal", "w-files"],
          shownWindowId: "w-terminal",
          rect: { x: 0, y: 0, w: 320, h: 560 },
        },
      ]}
      root={MAIN_STACK}
    />
  ),
};
