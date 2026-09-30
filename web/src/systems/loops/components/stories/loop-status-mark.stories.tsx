import type { Meta, StoryObj } from "@storybook/react-vite";

import { LOOP_RUN_LIVE_STATUSES, LOOP_RUN_TERMINAL_STATUSES } from "@/generated/loop-enums";
import { CenteredSurface } from "@/storybook/story-layout";

import { LoopStatusMark } from "../loop-status-mark";
import type { LoopRunStatus } from "../../types";

const meta: Meta<typeof LoopStatusMark> = {
  title: "systems/loops/components/LoopStatusMark",
  component: LoopStatusMark,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// The 12 first-class loop_run statuses: 5 live, then 7 terminal.
const LIVE_STATUSES = LOOP_RUN_LIVE_STATUSES satisfies readonly LoopRunStatus[];
const TERMINAL_STATUSES = LOOP_RUN_TERMINAL_STATUSES satisfies readonly LoopRunStatus[];

export const AllStatuses: Story = {
  render: () => (
    <CenteredSurface>
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-3">
          <span className="eyebrow text-muted">Live</span>
          <div className="flex flex-wrap items-center gap-2">
            {LIVE_STATUSES.map(status => (
              <LoopStatusMark key={status} status={status} />
            ))}
          </div>
        </div>
        <div className="flex flex-col gap-3">
          <span className="eyebrow text-muted">Terminal</span>
          <div className="flex flex-wrap items-center gap-2">
            {TERMINAL_STATUSES.map(status => (
              <LoopStatusMark key={status} status={status} />
            ))}
          </div>
        </div>
      </div>
    </CenteredSurface>
  ),
};

export const Running: Story = {
  render: () => (
    <CenteredSurface>
      <LoopStatusMark status="running" />
    </CenteredSurface>
  ),
};

export const Unknown: Story = {
  render: () => (
    <CenteredSurface>
      <LoopStatusMark status="something-else" />
    </CenteredSurface>
  ),
};
