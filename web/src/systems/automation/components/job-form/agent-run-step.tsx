import { Field, FieldLabel, Textarea } from "@agh/ui";

import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";

interface AgentRunStepProps {
  agent: string;
  agentDisabled?: boolean;
  agents: AgentPayload[];
  agentsLoading?: boolean;
  agentsError?: string | null;
  prompt: string;
  onAgentChange: (next: string) => void;
  onPromptChange: (next: string) => void;
}

/**
 * Agent output path: the agent to run and the plain prompt it receives verbatim.
 *
 * Agent selection goes through the shared searchable catalog, which carries
 * provider and category metadata — a native `<select>` here is forbidden
 * (`MODAL-STANDARD.md` § Component grammar).
 */
export function AgentRunStep({
  agent,
  agentDisabled = false,
  agents,
  agentsLoading = false,
  agentsError = null,
  prompt,
  onAgentChange,
  onPromptChange,
}: AgentRunStepProps) {
  return (
    <div className="space-y-4">
      <Field>
        <FieldLabel htmlFor="job-agent">Agent</FieldLabel>
        <AgentCommandSelect
          agents={agents}
          disabled={agentDisabled}
          error={agentsError}
          loading={agentsLoading}
          onChange={next => onAgentChange(next ?? "")}
          triggerId="job-agent"
          triggerTestId="job-agent-input"
          value={agent || null}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="job-prompt">Prompt</FieldLabel>
        <Textarea
          data-testid="job-prompt-input"
          id="job-prompt"
          onChange={event => onPromptChange(event.target.value)}
          value={prompt}
        />
        <p className="text-form-hint leading-snug text-subtle">
          Sent to the agent verbatim; jobs don&apos;t template, there&apos;s no event to
          interpolate.
        </p>
      </Field>
    </div>
  );
}
