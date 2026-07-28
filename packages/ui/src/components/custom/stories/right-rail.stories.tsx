import type { Meta, StoryObj } from "@storybook/react-vite";

import { RightRail } from "../right-rail";

const meta: Meta<typeof RightRail> = {
  title: "components/custom/RightRail",
  component: RightRail,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Right rail panel for thread overlays and channel inspectors. Fills its container (`h-full w-full`) so the parent — a `ResizablePanel` in the network shell — owns sizing. Left rule on `--line`, surface on `--canvas-soft`. Gracefully renders nothing when `open=false`.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="flex h-[400px] w-full bg-background border border-line">
        <div className="flex-1 p-4 text-[13px] text-muted">Main pane</div>
        <div className="w-(--width-right-rail-default) shrink-0">
          <Story />
        </div>
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Inspector mode — a typical channel inspector body.
 */
export const Inspector: Story = {
  args: {},
  render: () => (
    <RightRail open mode="inspector">
      <div className="border-b border-line px-4 py-3">
        <h2 className="text-[13px] font-medium text-fg-strong">Channel inspector</h2>
        <p className="text-[12px] text-muted">12 members · 3 active</p>
      </div>
      <div className="flex-1 overflow-y-auto px-4 py-3 text-[13px] text-muted">Inspector body</div>
    </RightRail>
  ),
};

/**
 * Thread mode — overlay with thread replies.
 */
export const Thread: Story = {
  args: {},
  render: () => (
    <RightRail open mode="thread">
      <div className="border-b border-line px-4 py-3">
        <h2 className="text-[13px] font-medium text-fg-strong">Thread</h2>
        <p className="text-[12px] text-muted">2 replies</p>
      </div>
      <div className="flex-1 overflow-y-auto px-4 py-3 text-[13px] text-muted">Thread replies</div>
    </RightRail>
  ),
};
