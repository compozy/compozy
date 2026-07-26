import type { Meta, StoryObj } from "@storybook/react-vite";
import { CheckIcon, GitCommitIcon, MessageSquareIcon } from "lucide-react";

import { Pill } from "@agh/ui";
import { Timeline } from "../timeline";
import { TimelineEvent } from "../timeline-event";

const meta: Meta<typeof Timeline> = {
  title: "components/custom/Timeline",
  component: Timeline,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Vertical timeline rail (`<ol>`) with a hairline `--line` spine running through the leading-icon column. Pair with `TimelineEvent` rows. Use for run histories, audit logs, network activity feeds.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Mixed-tone activity feed showing the full event composition.
 */
export const Activity: Story = {
  args: {},
  render: () => (
    <Timeline ariaLabel="Run activity">
      <TimelineEvent
        title="Run started"
        description="Anthropic Claude received the orchestration plan."
        time="04:21:15"
        tone="accent"
        icon={GitCommitIcon}
      />
      <TimelineEvent
        title="Reviewer flagged a regression risk"
        description="OpenAI requested a coverage test before merging."
        time="04:24:08"
        meta={<Pill tone="warning">Risk</Pill>}
        tone="warning"
        icon={MessageSquareIcon}
      />
      <TimelineEvent
        title="All checks passed"
        description="Three reviewers signed off on the diff."
        time="04:31:42"
        tone="success"
        icon={CheckIcon}
      />
    </Timeline>
  ),
};
