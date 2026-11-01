import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { cn } from "../../../lib/utils";
import { SEGMENT_PILL_TARGET } from "../hooks/use-segment-pill";
import { SegmentPill } from "../segment-pill";

const meta: Meta<typeof SegmentPill> = {
  title: "components/custom/SegmentPill",
  component: SegmentPill,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "The travelling selection plate of a segmented control. Mark each segment with `SEGMENT_PILL_TARGET` inside a positioned parent and pass the selected index (`null` hides it). The leading edge stretches toward the new segment and the trailing edge catches up — transform-only, so the radius never distorts; reduced motion jumps.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const SEGMENTS = ["Children 3", "Subagents 2/21", "All"] as const;

function SegmentHarness({ deselectable = false }: { deselectable?: boolean }) {
  const [index, setIndex] = useState<number | null>(0);
  return (
    <div role="group" aria-label="Lists" className="relative inline-flex items-stretch gap-0.5">
      {SEGMENTS.map((label, i) => (
        <button
          key={label}
          type="button"
          {...{ [SEGMENT_PILL_TARGET]: "" }}
          aria-pressed={index === i}
          onClick={() => setIndex(deselectable && index === i ? null : i)}
          className={cn(
            "relative z-10 h-6 rounded-pill px-2 font-mono text-micro tabular-nums outline-none focus-visible:shadow-focus-ring",
            index === i ? "text-fg" : "text-muted-foreground hover:text-fg"
          )}
        >
          {label}
        </button>
      ))}
      <SegmentPill index={index} />
    </div>
  );
}

export const Default: Story = {
  tags: ["play-fn"],
  render: () => <SegmentHarness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "All" }));
    await expect(canvas.getByRole("button", { name: "All" })).toHaveAttribute(
      "aria-pressed",
      "true"
    );
  },
};

export const Deselectable: Story = {
  tags: ["play-fn"],
  parameters: {
    docs: {
      description: {
        story:
          "Re-clicking the selected segment clears the selection; the plate fades out in place.",
      },
    },
  },
  render: () => <SegmentHarness deselectable />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Children 3" }));
    await waitFor(() =>
      expect(canvasElement.querySelector('[data-slot="segment-pill"]')).toHaveAttribute(
        "data-state",
        "hidden"
      )
    );
  },
};
