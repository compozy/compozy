import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { HoverCard, HoverCardContent, HoverCardTrigger } from "@compozy/ui";

import { SubagentCard } from "../subagent-card";
import { SubagentHoverContent, type SubagentHoverContentProps } from "../subagent-hover-content";
import { NO_RUNTIME, cardStates, settledResultLong } from "./subagent-story-fixtures";

function OpenHover({ label, ...props }: SubagentHoverContentProps & { label: string }) {
  return (
    <div className="h-80">
      <HoverCard defaultOpen>
        <HoverCardTrigger render={<span className="font-mono text-mono-id text-faint" />}>
          {label}
        </HoverCardTrigger>
        <HoverCardContent>
          <SubagentHoverContent {...props} />
        </HoverCardContent>
      </HoverCard>
    </div>
  );
}

/** Transcript VC-04: the hover card in its five content states, pinned open. */
function HoverStates() {
  return (
    <div className="grid max-w-5xl grid-cols-3 gap-x-8 p-8">
      <OpenHover
        label="live · progress preview"
        subagent={{ ...cardStates.running, progress: "Reading internal/payments/webhook.go" }}
      />
      <OpenHover label="settled · result 280 + …" subagent={settledResultLong} />
      <OpenHover label="failed · error text" subagent={cardStates.failed} />
      <OpenHover
        label="model not reported"
        subagent={{ ...cardStates.unknownProvider, runtime: { ...NO_RUNTIME, provider: "claude" } }}
      />
      <OpenHover
        label="worktree differs"
        subagent={cardStates.queued}
        location={{ worktree: "subagent/refund-reason-enum" }}
        parentLocation={{ worktree: "main" }}
      />
    </div>
  );
}

const meta: Meta = {
  title: "systems/session/components/subagents/HoverContent",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const States: Story = { render: () => <HoverStates /> };

/** 200 ms hover or focus opens it; Escape closes it. */
export const FocusOpensEscapeCloses: Story = {
  tags: ["play-fn"],
  render: () => (
    <div className="p-8">
      <SubagentCard subagent={cardStates.runningProgress} onOpen={() => undefined} />
    </div>
  ),
  play: async ({ canvasElement }) => {
    const card = await within(canvasElement).findByRole("button", {
      name: /^Open Audit payment webhooks/,
    });
    card.focus();
    const body = within(document.body);
    await waitFor(() => expect(body.getByText("Opus 5.5")).toBeInTheDocument());
    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(body.queryByText("Opus 5.5")).not.toBeInTheDocument());
  },
};
