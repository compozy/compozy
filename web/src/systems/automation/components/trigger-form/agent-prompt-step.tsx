import { Field, FieldLabel } from "@agh/ui";

import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";

import { PromptTemplateField } from "./prompt-template-field";

interface AgentPromptStepProps {
  agent: string;
  agentDisabled?: boolean;
  agents: AgentPayload[];
  agentsLoading?: boolean;
  agentsError?: string | null;
  prompt: string;
  variables: string[];
  onAgentChange: (next: string) => void;
  onPromptChange: (next: string) => void;
}

/**
 * "Then" step: the agent to run and the prompt template it receives.
 *
 * Agent selection goes through the shared searchable catalog; a native
 * `<select>` here is forbidden (`MODAL-STANDARD.md` § Component grammar).
 */
export function AgentPromptStep({
  agent,
  agentDisabled = false,
  agents,
  agentsLoading = false,
  agentsError = null,
  prompt,
  variables,
  onAgentChange,
  onPromptChange,
}: AgentPromptStepProps) {
  return (
    <div className="space-y-4">
      <Field>
        <FieldLabel htmlFor="trigger-agent">Agent</FieldLabel>
        <AgentCommandSelect
          agents={agents}
          disabled={agentDisabled}
          error={agentsError}
          loading={agentsLoading}
          onChange={next => onAgentChange(next ?? "")}
          triggerId="trigger-agent"
          triggerTestId="trigger-agent-input"
          value={agent || null}
        />
      </Field>
      <PromptTemplateField onChange={onPromptChange} value={prompt} variables={variables} />
    </div>
  );
}
