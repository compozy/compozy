/**
 * The Call action and the recent-calls list on agent detail.
 *
 * A connected edge: it owns the compose state and the two `agent`-filtered
 * reads, and hands the presentational compose panel a finished view model.
 *
 * The instance count renders only when it is greater than zero. "0 running" is
 * noise that reads like a fact, and an agent nobody is using should look calm
 * rather than explicitly idle.
 */
import { useNavigate } from "@tanstack/react-router";

import { Eyebrow, Pill } from "@compozy/ui";

import {
  AgentCallCompose,
  AgentCallStatePill,
  toCallState,
  useAgentCallCompose,
} from "@/systems/agent-comms";
import { useWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";

export function AgentCallComposeSection({
  agentName,
  windowId,
}: {
  agentName: string;
  windowId: string;
}) {
  const navigate = useNavigate({ from: "/agents" });
  const live = useWindowLiveDataEnabled(windowId);
  const compose = useAgentCallCompose(agentName, { live });

  const openCall = (callId: string) => {
    void navigate({ to: "/agents/calls/$callId", params: { callId } });
  };

  return (
    <section className="flex flex-col gap-3" data-testid="agent-detail-calls">
      <header className="flex items-center gap-2">
        <Eyebrow>Calls</Eyebrow>
        {compose.activeInstances !== undefined && compose.activeInstances > 0 ? (
          <Pill size="xs" tone="neutral" data-testid="agent-detail-active-instances">
            {compose.activeInstances} working
          </Pill>
        ) : null}
      </header>

      <AgentCallCompose
        data-testid="agent-detail-call-compose"
        agentName={agentName}
        prompt={compose.prompt}
        onPromptChange={compose.setPrompt}
        expect={compose.expect}
        onExpectChange={compose.setExpect}
        onSubmit={compose.submit}
        pending={compose.pending}
        failure={compose.failure}
        accepted={compose.accepted}
        onOpenAcceptedCall={openCall}
      />

      {compose.recentCallsPending ? (
        <p className="text-form text-fg-muted" data-testid="agent-detail-recent-calls-loading">
          Loading calls…
        </p>
      ) : compose.recentCallsError ? (
        <p className="text-form text-danger" data-testid="agent-detail-recent-calls-error">
          Calls unavailable
        </p>
      ) : compose.recentCalls.length > 0 ? (
        <ul
          className="flex flex-col divide-y divide-line-soft"
          data-testid="agent-detail-recent-calls"
        >
          {compose.recentCalls.map(call => (
            <li key={call.call_id} className="flex items-center gap-2 py-1.5">
              <button
                className="min-w-0 flex-1 truncate text-left text-form text-fg hover:underline"
                onClick={() => openCall(call.call_id)}
                type="button"
              >
                {call.caller.id}
              </button>
              <AgentCallStatePill state={toCallState(call.state)} fallbackLabel={call.state} />
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-form text-fg-muted" data-testid="agent-detail-recent-calls-empty">
          No calls yet
        </p>
      )}
    </section>
  );
}
