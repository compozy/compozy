import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";

import { AgentCallsInspectorPanel } from "../agent-calls-inspector-panel";
import { AgentCallTurnCard } from "../agent-call-turn-card";
import { AgentCallTurnFanout } from "../agent-call-turn-fanout";
import { AgentComposeMessage } from "../agent-compose-message";
import { AgentSyntheticTurn } from "../agent-synthetic-turn";
import type { SyntheticTurn } from "../../lib/synthetic-turn";
import {
  buildLargeTreeFixture,
  childMessageFixture,
  completedCallFixture,
  extractedCallFixture,
  failedMessageFixture,
  invalidResultCallFixture,
  operatorMessageFixture,
  queuedMessageFixture,
  runningCallFixture,
} from "../../mocks";
import type { CallMessagePayload, CallPayload } from "../../types";

/**
 * Transcript turns are read from `metadata.synthetic`, so a story that wants to
 * show one has to build the same shape the daemon writes. Rendering a fixture
 * through a different component would capture a surface the product does not
 * have.
 */
function emptySynthetic(kind: SyntheticTurn["kind"]): SyntheticTurn {
  return {
    kind,
    callId: null,
    callState: null,
    childSessionId: null,
    childAgentName: null,
    callerAgentName: null,
    resultBytes: null,
    contractDigest: null,
    requiredKeyCount: null,
    messageId: null,
    deliveryKind: null,
    reason: null,
    summary: null,
    verdict: null,
    wakeEventId: null,
  };
}

function messageTurn(message: CallMessagePayload): SyntheticTurn {
  return {
    ...emptySynthetic("message"),
    callId: message.call_id ?? null,
    messageId: message.message_id,
    deliveryKind: message.delivery,
    reason: message.reason ?? null,
    childAgentName: message.from_agent_name ?? null,
  };
}

function wakeTurn(call: CallPayload): SyntheticTurn {
  return {
    ...emptySynthetic("call-wake"),
    callId: call.call_id,
    callState: call.state,
    childSessionId: call.child_session_id ?? null,
    childAgentName: call.agent ?? null,
    resultBytes: call.result_bytes ?? null,
    contractDigest: call.expect_digest ?? null,
    summary: `Call completed: ${call.agent ?? "agent"} (${call.call_id}) → ${call.state}.`,
  };
}

const meta: Meta<typeof AgentSyntheticTurn> = {
  title: "systems/agent-comms/components/InContextMessages",
  component: AgentSyntheticTurn,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Messages render where they mean something — in the conversation they interrupted — with provenance, an untrusted frame, and a delivery receipt. There is no standalone inbox and no read state.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Every receipt a message can carry, including a failure with its reason. */
export const MessageTurns: Story = {
  render: () => (
    <PanelSurface>
      <div className="flex w-full max-w-2xl flex-col gap-4">
        {[
          childMessageFixture,
          operatorMessageFixture,
          queuedMessageFixture,
          failedMessageFixture,
        ].map(message => (
          <AgentSyntheticTurn
            key={message.message_id}
            turn={messageTurn(message)}
            text={message.text}
            timestamp={message.created_at}
          />
        ))}
      </div>
    </PanelSurface>
  ),
};

/** The resting card is its head row alone; the ask and answer wait for a click. */
export const CallTurnCards: Story = {
  render: () => (
    <PanelSurface>
      <div className="flex w-full max-w-2xl flex-col gap-2">
        <AgentCallTurnCard call={completedCallFixture} onOpenCall={() => undefined} />
        <AgentCallTurnCard call={runningCallFixture} onOpenCall={() => undefined} />
        <AgentCallTurnCard
          call={invalidResultCallFixture}
          defaultOpen
          onOpenCall={() => undefined}
        />
        <AgentCallTurnCard call={extractedCallFixture} defaultOpen onOpenCall={() => undefined} />
        <AgentCallTurnCard
          call={completedCallFixture}
          defaultOpen
          onOpenCall={() => undefined}
          prompt="Review the checkout retry path in HEAD~1..HEAD"
        />
      </div>
    </PanelSurface>
  ),
};

/** Many agents at once stay one row, with the worst state escalated to the head. */
export const FanOut: Story = {
  render: () => (
    <PanelSurface>
      <div className="flex w-full max-w-2xl flex-col gap-2">
        <AgentCallTurnFanout calls={buildLargeTreeFixture(12)} onOpenCall={() => undefined} />
        <AgentCallTurnFanout
          calls={buildLargeTreeFixture(12)}
          defaultOpen
          onOpenCall={() => undefined}
          onOpenCallsPanel={() => undefined}
        />
      </div>
    </PanelSurface>
  ),
};

/** Why this session woke — the wake's own words, not a rephrasing. */
export const WakeCards: Story = {
  render: () => (
    <PanelSurface>
      <div className="flex w-full max-w-2xl flex-col gap-3">
        {[completedCallFixture, invalidResultCallFixture].map(call => (
          <AgentSyntheticTurn
            key={call.call_id}
            turn={wakeTurn(call)}
            text={`Call completed: ${call.agent ?? "agent"} (${call.call_id}) → ${call.state}.`}
          />
        ))}
      </div>
    </PanelSurface>
  ),
};

/** The child's bound plate and the return receipt. */
export const HelperSession: Story = {
  render: () => (
    <PanelSurface>
      <div className="flex w-full max-w-2xl flex-col gap-3">
        <AgentSyntheticTurn
          text={`Call context: stay in the asked files
Review the checkout retry path in HEAD~1..HEAD`}
          turn={{
            ...emptySynthetic("call-request"),
            callId: completedCallFixture.call_id,
            callerAgentName: "planner",
            contractDigest: "sha256:9f2c",
            requiredKeyCount: 2,
          }}
        />
        <AgentSyntheticTurn
          text=""
          turn={{
            ...emptySynthetic("call-return"),
            callId: completedCallFixture.call_id,
            callerAgentName: "planner",
            verdict: "returned",
          }}
        />
      </div>
    </PanelSurface>
  ),
};

/** The compose flow's four moments, including two typed refusals. */
export const ComposeMessage: Story = {
  render: () => (
    <PanelSurface>
      <div className="flex w-full max-w-2xl flex-col gap-3">
        <AgentComposeMessage
          onChange={() => undefined}
          onSend={() => undefined}
          targetLabel="compliance-review-agent"
          value="Prioritize the checkout retry path first."
        />
        <AgentComposeMessage
          failureCode="message_target_blocked"
          onChange={() => undefined}
          onSend={() => undefined}
          targetLabel="compliance-review-agent"
          value="Proceed anyway."
        />
        <AgentComposeMessage
          failureCode="message_rate_limited"
          onChange={() => undefined}
          onSend={() => undefined}
          targetLabel="compliance-review-agent"
          value="Ping."
        />
        <AgentComposeMessage
          accepted={{ messageId: "msg_01JBD8M2R4V7", delivery: "queued" }}
          onChange={() => undefined}
          onSend={() => undefined}
          targetLabel="compliance-review-agent"
          value=""
        />
      </div>
    </PanelSurface>
  ),
};

/** Both directions, with counts that come from the daemon rather than the rows. */
export const CallsInspectorPanel: Story = {
  render: () => (
    <PanelSurface>
      <div className="w-full max-w-md">
        <AgentCallsInspectorPanel
          made={{
            calls: buildLargeTreeFixture(25),
            total: 247,
            hasMore: true,
            onLoadMore: () => undefined,
          }}
          onOpenCall={() => undefined}
          received={{
            calls: [runningCallFixture],
            total: 1,
            hasMore: false,
            onLoadMore: () => undefined,
          }}
        />
      </div>
    </PanelSurface>
  ),
};

/** Retention pruned the counterpart: identity stays, the link degrades. */
export const PrunedCounterpart: Story = {
  render: () => (
    <PanelSurface>
      <div className="w-full max-w-md">
        <AgentCallsInspectorPanel
          made={{
            calls: [completedCallFixture],
            total: 1,
            hasMore: false,
            onLoadMore: () => undefined,
          }}
          onOpenCall={() => undefined}
          prunedSessionIds={new Set([completedCallFixture.child_session_id ?? ""])}
          received={{ calls: [], total: 0, hasMore: false, onLoadMore: () => undefined }}
        />
      </div>
    </PanelSurface>
  ),
};
