import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState, type ComponentProps } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import type { DesktopIndicatorItem } from "../../lib/desktop-indicator-controls";
import { DesktopIndicators } from "../desktop-indicators";
import { DesktopShell } from "./_desktop";

const DESKTOPS: DesktopIndicatorItem[] = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review" },
  { id: "research", name: "Research" },
];

/** Research holds a session that needs you: an amber dot until it is the active desktop. */
const NEEDS_YOU_DESKTOPS: DesktopIndicatorItem[] = DESKTOPS.map(desktop =>
  desktop.id === "research" ? { ...desktop, needsYou: true } : desktop
);

const MANY_DESKTOPS: DesktopIndicatorItem[] = [
  ...DESKTOPS,
  { id: "qa", name: "QA" },
  { id: "docs", name: "Docs" },
  { id: "release", name: "Release" },
  { id: "incidents", name: "Incidents" },
  { id: "archive", name: "Archive" },
];

type IndicatorsProps = ComponentProps<typeof DesktopIndicators>;

function InteractiveIndicators({
  compact = false,
  ...args
}: IndicatorsProps & { compact?: boolean }) {
  const [activeDesktopId, setActiveDesktopId] = useState(args.activeDesktopId);
  return (
    <DesktopShell
      wallpaper="carbon"
      deskHint
      compact={compact}
      pager={
        <DesktopIndicators
          {...args}
          activeDesktopId={activeDesktopId}
          onSelectDesktop={desktopId => {
            setActiveDesktopId(desktopId);
            args.onSelectDesktop(desktopId);
          }}
        />
      }
    />
  );
}

const meta: Meta<typeof DesktopIndicators> = {
  title: "systems/os/components/DesktopIndicators",
  component: DesktopIndicators,
  args: {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    onSelectDesktop: fn(),
    onOpenOverview: fn(),
    onRenameDesktop: fn(),
    onReorderDesktop: fn(),
    onCloseDesktop: fn(),
    onCreateDesktop: fn(),
  },
  render: args => <InteractiveIndicators {...args} />,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Top-bar desktop indicators: 6px dots, the active desktop as an 18px pill, an amber dot for an off-screen desktop that needs you. A click switches; right-click or Shift+F10 opens Rename, Move left, Move right, Close desktop and New desktop; past seven desktops, overflow controls open the overview. While a tab is dragged the indicators grow into labelled drop targets.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** `indicator-rest`: four desktops stay fully visible; a click moves the active pill. */
export const Rest: Story = {};

/** `indicator-menu`: the menu of a middle desktop has every item. */
export const Menu: Story = {
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.pointer({
      keys: "[MouseRight]",
      target: canvas.getByRole("button", { name: "Desktop 2 of 4: Build" }),
    });
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByRole("menuitem", { name: "Move left" })).toBeVisible();
    await expect(body.getByRole("menuitem", { name: "Close desktop" })).toBeVisible();
  },
};

/** `indicator-menu-first`: the first desktop's menu has no Move left. */
export const MenuOnFirstDesktop: Story = {
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.pointer({
      keys: "[MouseRight]",
      target: canvas.getByRole("button", { name: "Desktop 1 of 4: Control" }),
    });
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByRole("menuitem", { name: "Move right" })).toBeVisible();
    await expect(body.queryByRole("menuitem", { name: "Move left" })).toBeNull();
  },
};

/** `indicator-targets`: a tab from Build is dragged; every desktop is a labelled drop target. */
export const DropTargets: Story = { args: { dragSourceDesktopId: "build" } };

/** `indicator-drop`: the dragged tab is over Review and moves there on release. */
export const DropOverDesktop: Story = {
  args: { dragSourceDesktopId: "build", dropTargetDesktopId: "review" },
};

/** `indicator-overflow`: nine desktops show five around the active one and both hidden ranges. */
export const Overflow: Story = { args: { desktops: MANY_DESKTOPS, activeDesktopId: "qa" } };

/** `indicator-overflow-drop`: the dragged tab rests on the later overflow, opening the overview. */
export const OverflowDrop: Story = {
  args: {
    desktops: MANY_DESKTOPS,
    activeDesktopId: "qa",
    dragSourceDesktopId: "qa",
    overflowHover: "later",
  },
};

/** The adaptive window hugs the final desktop without wrapping or empty positions. */
export const OverflowAtEnd: Story = {
  args: { desktops: MANY_DESKTOPS, activeDesktopId: "archive" },
};

/** An off-screen desktop that needs you shows an amber dot; switching to it clears the mark. */
export const NeedsYouOffScreen: Story = { args: { desktops: NEEDS_YOU_DESKTOPS } };

/** Compact presentation (tab bar below, narrow top bar) keeps the same window around the active desktop. */
export const CompactTabBar: Story = {
  args: { desktops: MANY_DESKTOPS, activeDesktopId: "qa" },
  render: args => <InteractiveIndicators {...args} compact />,
};
