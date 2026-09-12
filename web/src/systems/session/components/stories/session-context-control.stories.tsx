import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { SessionContextControl } from "../session-context-control";
import { deriveSessionContext } from "../../lib/session-context";
import { sessionContextFixture } from "../../mocks/context-fixtures";

const meta = {
  title: "systems/session/components/SessionContextControl",
  component: SessionContextControl,
  parameters: { layout: "centered" },
  args: { context: deriveSessionContext(sessionContextFixture), onOpen: fn() },
} satisfies Meta<typeof SessionContextControl>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Reported: Story = {};
export const Warning: Story = {
  args: { context: deriveSessionContext({ ...sessionContextFixture, used: 225_280, ratio: 0.88 }) },
};
export const Unknown: Story = { args: { context: deriveSessionContext() } };
export const Unavailable: Story = {
  args: { context: deriveSessionContext(sessionContextFixture, { unavailable: true }) },
};
export const FreshUnavailable: Story = {
  args: { context: deriveSessionContext(undefined, { unavailable: true }) },
};
export const EstimatedSize: Story = {
  args: {
    context: deriveSessionContext({
      ...sessionContextFixture,
      state: "estimated_size",
      size_source: "catalog",
      pressure_threshold: undefined,
    }),
  },
};
export const UsedOnly: Story = {
  args: {
    context: deriveSessionContext({ state: "reported", used: 89_700, reported_turn_id: "turn-12" }),
  },
};
export const Stale: Story = {
  args: { context: deriveSessionContext({ ...sessionContextFixture, stale: true }) },
};
export const Stopped: Story = {
  args: { context: deriveSessionContext(sessionContextFixture, { stopped: true }) },
};
export const Loading: Story = {
  args: { context: deriveSessionContext(undefined, { loading: true }) },
};
export const OverCapacity: Story = {
  args: { context: deriveSessionContext({ ...sessionContextFixture, used: 281_600, ratio: 1.1 }) },
};
/** The Context rail is open: the trigger keeps its plate while sharing the rail's preference. */
export const RailOpen: Story = { args: { open: true } };
export const NarrowRow: Story = {
  decorators: [
    Story => (
      <div className="flex w-60 flex-wrap items-center gap-2 rounded-xl border border-line p-2">
        <span className="text-small-body text-muted">Claude Code</span>
        <span className="text-small-body text-muted">Main worktree</span>
        <Story />
      </div>
    ),
  ],
};

export const StaleEstimatedSize: Story = {
  args: {
    context: deriveSessionContext({
      ...sessionContextFixture,
      stale: true,
      state: "estimated_size",
      size_source: "catalog",
      pressure_threshold: undefined,
    }),
  },
};
export const StoppedOverCapacity: Story = {
  args: {
    context: deriveSessionContext(
      { ...sessionContextFixture, used: 281_600, ratio: 1.1 },
      { stopped: true }
    ),
  },
};
