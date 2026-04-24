import type { Meta, StoryObj } from "@storybook/react-vite";

import { StatusBadge } from "../status-badge";
import {
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "../surface-card";

const meta: Meta<typeof SurfaceCard> = {
  title: "ui/SurfaceCard",
  component: SurfaceCard,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Primary container for metrics, lists, and operator callouts.",
      },
    },
  },
  tags: ["autodocs"],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Canonical daemon card composition with header, body, and footer metadata.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <SurfaceCard className="w-[32rem]">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Queue</SurfaceCardEyebrow>
          <SurfaceCardTitle>Run queue</SurfaceCardTitle>
          <SurfaceCardDescription>
            Snapshot of queued and completed runs for the active workspace.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">total 7</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        <div className="grid grid-cols-2 gap-3">
          <div className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2">
            <p className="eyebrow text-muted-foreground">active</p>
            <p className="mt-1 font-mono text-2xl text-foreground tabular-nums">1</p>
          </div>
          <div className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2">
            <p className="eyebrow text-muted-foreground">failed</p>
            <p className="mt-1 font-mono text-2xl text-foreground tabular-nums">1</p>
          </div>
        </div>
      </SurfaceCardBody>
      <SurfaceCardFooter>
        <span className="text-xs text-muted-foreground">updated just now</span>
        <StatusBadge tone="accent">live</StatusBadge>
      </SurfaceCardFooter>
    </SurfaceCard>
  ),
};
