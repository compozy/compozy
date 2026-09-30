import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState, type ComponentProps } from "react";
import { fn } from "storybook/test";

import { DesktopPager, type DesktopPagerItem } from "../desktop-pager";
import { DesktopShell } from "./_desktop";

const DESKTOPS: DesktopPagerItem[] = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review" },
  { id: "research", name: "Research" },
];

/** Research holds a session that needs you: an orange dot until it is the active desktop. */
const NEEDS_YOU_DESKTOPS: DesktopPagerItem[] = DESKTOPS.map(desktop =>
  desktop.id === "research" ? { ...desktop, needsYou: true } : desktop
);

const MANY_DESKTOPS: DesktopPagerItem[] = [
  ...DESKTOPS,
  { id: "qa", name: "QA" },
  { id: "docs", name: "Docs" },
  { id: "release", name: "Release" },
  { id: "incidents", name: "Incidents" },
  { id: "archive", name: "Archive" },
];

function InteractivePager({
  desktops,
  initialDesktopId,
  compact = false,
  onSelectDesktop,
  onOpenOverview,
}: {
  desktops: readonly DesktopPagerItem[];
  initialDesktopId: string;
  compact?: boolean;
  onSelectDesktop: (desktopId: string) => void;
  onOpenOverview: ComponentProps<typeof DesktopPager>["onOpenOverview"];
}) {
  const [activeDesktopId, setActiveDesktopId] = useState(initialDesktopId);

  return (
    <DesktopShell
      wallpaper="carbon"
      deskHint
      compact={compact}
      pager={
        <DesktopPager
          desktops={desktops}
          activeDesktopId={activeDesktopId}
          onSelectDesktop={desktopId => {
            setActiveDesktopId(desktopId);
            onSelectDesktop(desktopId);
          }}
          onOpenOverview={onOpenOverview}
        />
      }
    />
  );
}

const meta: Meta<typeof DesktopPager> = {
  title: "systems/os/components/DesktopPager",
  component: DesktopPager,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Topbar desktop navigation in the tray: 6px dots, the active desktop as an 18px pill, an orange dot for an off-screen desktop that needs you, arrow-key navigation, and ±2 overflow controls that open the desktops overview. Dots only select.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Four desktops stay fully visible; selecting a dot updates the active pill. */
export const AllVisible: Story = {
  args: {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    onSelectDesktop: fn(),
    onOpenOverview: fn(),
  },
  render: args => (
    <InteractivePager
      desktops={args.desktops}
      initialDesktopId={args.activeDesktopId}
      onSelectDesktop={args.onSelectDesktop}
      onOpenOverview={args.onOpenOverview}
    />
  ),
};

/** Nine desktops collapse around the active desktop and expose both hidden ranges. */
export const OverflowMiddle: Story = {
  args: {
    desktops: MANY_DESKTOPS,
    activeDesktopId: "qa",
    onSelectDesktop: fn(),
    onOpenOverview: fn(),
  },
  render: args => (
    <InteractivePager
      desktops={args.desktops}
      initialDesktopId={args.activeDesktopId}
      onSelectDesktop={args.onSelectDesktop}
      onOpenOverview={args.onOpenOverview}
    />
  ),
};

/** The adaptive window hugs the final desktop without wrapping or empty positions. */
export const OverflowAtEnd: Story = {
  args: {
    desktops: MANY_DESKTOPS,
    activeDesktopId: "archive",
    onSelectDesktop: fn(),
    onOpenOverview: fn(),
  },
  render: args => (
    <InteractivePager
      desktops={args.desktops}
      initialDesktopId={args.activeDesktopId}
      onSelectDesktop={args.onSelectDesktop}
      onOpenOverview={args.onOpenOverview}
    />
  ),
};

/** An off-screen desktop that needs you shows an orange dot; switching to it clears the mark. */
export const NeedsYouOffScreen: Story = {
  args: {
    desktops: NEEDS_YOU_DESKTOPS,
    activeDesktopId: "build",
    onSelectDesktop: fn(),
    onOpenOverview: fn(),
  },
  render: args => (
    <InteractivePager
      desktops={args.desktops}
      initialDesktopId={args.activeDesktopId}
      onSelectDesktop={args.onSelectDesktop}
      onOpenOverview={args.onOpenOverview}
    />
  ),
};

/** Compact presentation (tab bar below, narrow topbar) keeps the same ±2 window around the active desktop. */
export const CompactTabBar: Story = {
  args: {
    desktops: MANY_DESKTOPS,
    activeDesktopId: "qa",
    onSelectDesktop: fn(),
    onOpenOverview: fn(),
  },
  render: args => (
    <InteractivePager
      desktops={args.desktops}
      initialDesktopId={args.activeDesktopId}
      compact
      onSelectDesktop={args.onSelectDesktop}
      onOpenOverview={args.onOpenOverview}
    />
  ),
};
