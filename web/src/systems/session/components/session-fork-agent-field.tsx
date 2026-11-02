import { Field, FieldLabel } from "@compozy/ui";

import { AgentIdentityChip } from "@/systems/agent/components/agent-identity-mark";

export interface SessionForkAgentFieldProps {
  agentName: string;
  provider: string;
}

/** The agent, read-only: a fork is this agent; another agent is a Continue. */
export function SessionForkAgentField({ agentName, provider }: SessionForkAgentFieldProps) {
  return (
    <Field>
      <FieldLabel id="session-fork-agent-label">Agent</FieldLabel>
      <div
        aria-labelledby="session-fork-agent-label"
        className="flex min-h-input min-w-0 items-center gap-2 rounded-md border border-line-soft px-2.5 text-form-input text-fg"
        data-testid="session-fork-agent"
        role="group"
      >
        <AgentIdentityChip name={agentName} size={20} />
        {provider ? (
          <>
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
            <span className="truncate font-mono text-eyebrow text-faint">{provider}</span>
          </>
        ) : null}
      </div>
    </Field>
  );
}
