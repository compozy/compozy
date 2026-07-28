import { useEffect, useRef } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { ThinkingBlock } from "../thinking-block";

const meta: Meta<typeof ThinkingBlock> = {
  title: "systems/session/components/ThinkingBlock",
  component: ThinkingBlock,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const REASONING_MARKDOWN = [
  "I need the approved pricing language and the partner-bank fallback copy before",
  "closing the launch checklist. Working plan:",
  "",
  "- confirm the hero banner claim against `COPY.md`",
  "- verify the `$0 setup` line has an approved source",
  "- swap the fallback state if the partner feed is stale",
  "",
  "```ts",
  "const ready = pricingApproved && fallbackCopyResolved;",
  "```",
].join("\n");

function ThinkingFrame({ children }: { children: React.ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-2xl rounded-md border border-line bg-canvas py-2 pr-2 pl-1.5">
        {children}
      </div>
    </CenteredSurface>
  );
}

function AutoExpanded({ updateCount }: { updateCount?: number }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      containerRef.current
        ?.querySelector<HTMLButtonElement>("[data-testid='thinking-trigger']")
        ?.click();
    });
    return () => window.cancelAnimationFrame(frame);
  }, []);

  return (
    <div ref={containerRef}>
      <ThinkingBlock thinking={REASONING_MARKDOWN} thinkingComplete updateCount={updateCount} />
    </div>
  );
}

export const Collapsed: Story = {
  render: () => (
    <ThinkingFrame>
      <ThinkingBlock thinking={REASONING_MARKDOWN} thinkingComplete />
    </ThinkingFrame>
  ),
};

export const Expanded: Story = {
  render: () => (
    <ThinkingFrame>
      <AutoExpanded />
    </ThinkingFrame>
  ),
};

// Grouped reasoning: consecutive updates collapse into one row with an "N updates"
// count in the trigger, expanding to the same indented markdown rail.
export const Grouped: Story = {
  render: () => (
    <ThinkingFrame>
      <AutoExpanded updateCount={4} />
    </ThinkingFrame>
  ),
};

// Live reasoning: auto-open with the typing-dots indicator while the turn streams.
export const Streaming: Story = {
  render: () => (
    <ThinkingFrame>
      <ThinkingBlock thinking={REASONING_MARKDOWN} thinkingComplete={false} />
    </ThinkingFrame>
  ),
};
