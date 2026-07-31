import {
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FormSection,
  RequiredMark,
} from "@compozy/ui";

import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";

interface SessionCreateSimpleSectionProps {
  agents: AgentPayload[];
  selectedAgentName: string;
  onAgentChange: (agentName: string) => void;
  /** Resolved from the workspace payload by the dialog — the single source of truth. */
  workspaceSelected: boolean;
  isSubmitting: boolean;
}

/**
 * Simple tier of the session launcher: choose the agent. Workspace and launch
 * details remain available as the single Advanced disclosure tier.
 */
function SessionCreateSimpleSection({
  agents,
  selectedAgentName,
  onAgentChange,
  workspaceSelected,
  isSubmitting,
}: SessionCreateSimpleSectionProps) {
  const trimmedSelectedAgentName = selectedAgentName.trim();
  const hasAgents = agents.length > 0;
  const agentPlaceholder = !workspaceSelected
    ? "Select a workspace first"
    : hasAgents
      ? "Select an agent"
      : "No agents available";

  return (
    <FormSection
      description="The agent brings its configured instructions and tools."
      title="Agent"
    >
      <div className="flex flex-col gap-4">
        <Field>
          <FieldContent>
            <FieldLabel htmlFor="session-create-agent">
              Agent
              <RequiredMark />
            </FieldLabel>
            <FieldDescription>
              The agent owns the instructions and tools for this session.
            </FieldDescription>
          </FieldContent>
          <AgentCommandSelect
            agents={agents}
            disabled={!workspaceSelected || !hasAgents || isSubmitting}
            onChange={next => onAgentChange(next ?? "")}
            placeholder={agentPlaceholder}
            triggerId="session-create-agent"
            triggerTestId="session-create-agent-select"
            value={workspaceSelected ? trimmedSelectedAgentName || null : null}
          />
        </Field>
      </div>
    </FormSection>
  );
}

export { SessionCreateSimpleSection };
