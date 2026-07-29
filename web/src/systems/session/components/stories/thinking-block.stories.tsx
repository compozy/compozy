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

function AutoExpanded() {
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
      <ThinkingBlock thinking={REASONING_MARKDOWN} thinkingComplete />
    </div>
  );
}

// Settled reasoning rests as a tool-row line: "Thought" verb + first-line preview.
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

// Live reasoning: a shimmering "Thinking…" label with the streaming body
// auto-opened beneath it — no icon well, no dots.
export const Streaming: Story = {
  render: () => (
    <ThinkingFrame>
      <ThinkingBlock thinking={REASONING_MARKDOWN} thinkingComplete={false} />
    </ThinkingFrame>
  ),
};
