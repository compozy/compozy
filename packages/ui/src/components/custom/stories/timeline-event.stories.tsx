import type { Meta, StoryObj } from "@storybook/react-vite";
import { GitCommitIcon, ZapIcon } from "lucide-react";

import { Pill } from "@agh/ui";
import { Timeline } from "../timeline";
import { TimelineEvent } from "../timeline-event";

const meta: Meta<typeof TimelineEvent> = {
  title: "components/custom/TimelineEvent",
  component: TimelineEvent,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Single event row inside a `Timeline`. Composes a leading marker (icon or tone dot), title + optional description + meta, and a trailing mono time stamp. Tone drives only the marker dot color, never the surface — keeps the rail quiet.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4">
        <Timeline>
          <Story />
        </Timeline>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Event with a tone dot marker (no icon).
 */
export const ToneDot: Story = {
  args: {},
  render: () => (
    <TimelineEvent
      title="Workspace bootstrapped"
      description="Created `personal` workspace with 3 connected providers."
      time="2m"
      tone="accent"
    />
  ),
};

/**
 * Event with an icon marker, status pill in meta, and a long description.
 */
export const WithIconAndMeta: Story = {
  args: {},
  render: () => (
    <TimelineEvent
      title="Capability registered"
      description="`network.publish` is now callable via the agh-network/v0 protocol."
      time="04:21:15"
      meta={
        <>
          <Pill tone="success">Live</Pill>
          <span>kind=capability</span>
        </>
      }
      icon={ZapIcon}
      tone="success"
    />
  ),
};

/**
 * Marker-less event for compact log entries.
 */
export const NoMarker: Story = {
  args: {},
  render: () => (
    <TimelineEvent
      title="System note"
      description="Connection re-established after a brief network blip."
      time="04:25:01"
      hasMarker={false}
    />
  ),
};

/**
 * Combination — multiple events to verify the rail layout.
 */
export const Composed: Story = {
  args: {},
  render: () => (
    <>
      <TimelineEvent title="Session opened" time="04:00" tone="accent" icon={GitCommitIcon} />
      <TimelineEvent
        title="First message"
        description="User: refactor agh-network/v0"
        time="04:01"
        tone="neutral"
      />
      <TimelineEvent title="Done" time="04:42" tone="success" />
    </>
  ),
};
