import { Field, FieldLabel, OwnerAvatar } from "@compozy/ui";

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
        <OwnerAvatar name={agentName} ownerId={agentName} ownerKind="agent" size="sm" />
        <span className="truncate">{agentName}</span>
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
