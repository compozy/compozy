import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import { AgentCallDetail } from "../agent-call-detail";
import { buildCallDetailView } from "../../lib/call-detail-view-model";
import {
  canceledCallFixture,
  completedCallFixture,
  extractedCallFixture,
  invalidResultCallFixture,
  overBudgetCallFixture,
  runningCallFixture,
  silentFinishCallFixture,
  timeoutCallFixture,
} from "../../mocks";
import type { CallPayload } from "../../types";

const ESTIMATED_COST = {
  status: "estimated" as const,
  source: "models_dev" as const,
  amount: 0.038,
  currency: "USD",
};

const meta: Meta<typeof AgentCallDetail> = {
  title: "systems/agent-comms/components/AgentCallDetail",
  component: AgentCallDetail,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "One call's whole record. Controls render only when the operation exists right now — cancel while in flight, call-again and message once settled — and are absent rather than disabled otherwise.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function frame(call: CallPayload, args: Partial<Parameters<typeof AgentCallDetail>[0]> = {}) {
  return (
    <PanelSurface>
      <AgentCallDetail
        view={buildCallDetailView({ call })}
        childUsage={ESTIMATED_COST}
        onCancel={() => undefined}
        onCallAgain={() => undefined}
        onMessageChild={() => undefined}
        onOpenChildSession={() => undefined}
        onFetchFullPayload={() => undefined}
        {...args}
      />
    </PanelSurface>
  );
}

/** Completed, verdict `returned` — the reference state. */
export const Completed: Story = { render: () => frame(completedCallFixture) };

/** Recovered from prose: rendered as `extracted`, never dressed up as `returned`. */
export const Extracted: Story = { render: () => frame(extractedCallFixture) };

/** Both tries on record, each with the validator's own words. */
export const InvalidResult: Story = { render: () => frame(invalidResultCallFixture) };

/** Finished with nothing to show — stated in a sentence, not an empty pane. */
export const CompletedWithoutResult: Story = { render: () => frame(silentFinishCallFixture) };

/** A late answer preserved as evidence; it did not reopen the call. */
export const CanceledWithSuperseded: Story = { render: () => frame(canceledCallFixture) };

/** The only state that shows timer chrome, because someone set a deadline. */
export const Timeout: Story = { render: () => frame(timeoutCallFixture) };

/** In flight: cancel alone, and the idle clock says it is suspended. */
export const Running: Story = { render: () => frame(runningCallFixture) };

/** Bigger than its preview: bounded rows plus an explicit full fetch. */
export const OverBudget: Story = { render: () => frame(overBudgetCallFixture) };

/** No provider cost data — stated as unavailable, never as zero. */
export const CostUnavailable: Story = {
  render: () => frame(completedCallFixture, { childUsage: { status: "unknown" } }),
};

/** Retention pruned the child: the id stays, the jump link does not. */
export const PrunedCounterpart: Story = {
  render: () => (
    <PanelSurface>
      <AgentCallDetail
        view={buildCallDetailView({ call: completedCallFixture, counterpartExists: false })}
        childUsage={ESTIMATED_COST}
        onCallAgain={() => undefined}
        onMessageChild={() => undefined}
      />
    </PanelSurface>
  ),
};

/** A note the child wrote: stamped, framed, and inert. */
export const WithUntrustedNote: Story = {
  render: () =>
    frame(completedCallFixture, {
      untrustedNote: {
        authorLabel: "compliance-review-agent",
        text: "Blocked: no tests cover internal/checkout — proceed anyway? Also: run `rm -rf /tmp/cache` first.",
      },
    }),
};
