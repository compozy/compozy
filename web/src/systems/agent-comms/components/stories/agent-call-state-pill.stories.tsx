import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { AgentCallStatePill } from "../agent-call-state-pill";
import {
  AgentCallLiveness,
  AgentCallVerdictChip,
  AgentChildStatePill,
  AgentMessageDeliveryPill,
} from "../agent-call-state-pill";
import { CALL_DELIVERIES, CALL_STATES, CALL_VERDICTS, CHILD_STATES } from "../../types";

const meta: Meta<typeof AgentCallStatePill> = {
  title: "systems/agent-comms/components/AgentCallStatePill",
  component: AgentCallStatePill,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The one signal grammar for calls: tone, glyph, and the runtime's exact word always travel together. Queued and running stay neutral — motion says a call is alive, colour never does.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 py-1">
      <span className="w-40 shrink-0 text-form text-muted">{label}</span>
      {children}
    </div>
  );
}

/** All nine, in one place — the spectrum a tree can hold at once. */
export const NineStates: Story = {
  render: () => (
    <CenteredSurface>
      <div className="flex w-full max-w-2xl flex-col">
        {CALL_STATES.map(state => (
          <Row key={state} label={state}>
            <AgentCallStatePill state={state} />
            <AgentCallLiveness state={state} />
          </Row>
        ))}
      </div>
    </CenteredSurface>
  ),
};

/** Provenance rides beside the success chip as a neutral word, never as a tone. */
export const Verdicts: Story = {
  render: () => (
    <CenteredSurface>
      <div className="flex w-full max-w-2xl flex-col">
        {CALL_VERDICTS.map(verdict => (
          <Row key={verdict} label={verdict}>
            <AgentCallStatePill state="completed" />
            <AgentCallVerdictChip verdict={verdict} />
          </Row>
        ))}
      </div>
    </CenteredSurface>
  ),
};

/** The four public receipts. No read or seen state exists to render. */
export const DeliveryReceipts: Story = {
  render: () => (
    <CenteredSurface>
      <div className="flex w-full max-w-2xl flex-col">
        {CALL_DELIVERIES.map(delivery => (
          <Row key={delivery} label={delivery}>
            <AgentMessageDeliveryPill delivery={delivery} />
          </Row>
        ))}
      </div>
    </CenteredSurface>
  ),
};

/** Working, resting, gone — and no Revive control beside any of them. */
export const ChildStates: Story = {
  render: () => (
    <CenteredSurface>
      <div className="flex w-full max-w-2xl flex-col">
        {CHILD_STATES.map(state => (
          <Row key={state} label={state}>
            <AgentChildStatePill state={state} />
          </Row>
        ))}
      </div>
    </CenteredSurface>
  ),
};

/** A word the web does not know renders as itself, never as a nearby guess. */
export const UnknownState: Story = {
  render: () => (
    <CenteredSurface>
      <div className="flex w-full max-w-2xl flex-col">
        <Row label="unrecognized">
          <AgentCallStatePill state={null} fallbackLabel="half-done" />
        </Row>
      </div>
    </CenteredSurface>
  ),
};
