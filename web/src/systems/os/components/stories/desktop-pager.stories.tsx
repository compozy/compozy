import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState, type ComponentProps } from "react";
import { fn } from "storybook/test";

import { DesktopPager, type DesktopPagerItem } from "../desktop-pager";
import { OsDockZone } from "../os-dock";
import { OsDockTabBar } from "../os-dock-tab-bar";
import { DESK_ITEMS, DesktopShell } from "./_desktop";

const DESKTOPS: DesktopPagerItem[] = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review" },
  { id: "research", name: "Research" },
];

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
    <DesktopShell wallpaper="carbon" deskHint dock={false}>
      {compact ? (
        <OsDockTabBar
          items={DESK_ITEMS}
          leading={
            <DesktopPager
              desktops={desktops}
              activeDesktopId={activeDesktopId}
              compact
              onSelectDesktop={desktopId => {
                setActiveDesktopId(desktopId);
                onSelectDesktop(desktopId);
              }}
              onOpenOverview={onOpenOverview}
            />
          }
          onSelect={fn()}
          onNewSession={fn()}
        />
      ) : (
        <OsDockZone
          items={DESK_ITEMS}
          leading={
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
          onSelect={fn()}
          onNewSession={fn()}
        />
      )}
    </DesktopShell>
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
          "Bottom-chrome desktop navigation with invisible 44px targets, minimal position dots, keyboard navigation, and adaptive overflow into the management overview.",
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

/** Compact tab-bar mode keeps the active desktop between at most two overflow controls. */
export const CompactTabBar: Story = {
  args: {
    desktops: MANY_DESKTOPS,
    activeDesktopId: "qa",
    compact: true,
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
