import {
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
  FormSection,
  Input,
  RequiredMark,
} from "@agh/ui";

import {
  AgentCommandSelect,
  AgentIcon,
  resolveAgentRuntimeValue,
  type AgentPayload,
} from "@/systems/agent";
import { WorkspaceCommandSelect, type WorkspaceCommandSelectOption } from "@/systems/workspace";

interface SessionCreateSimpleSectionProps {
  agents: AgentPayload[];
  selectedAgentName: string;
  onAgentChange: (agentName: string) => void;
  workspaces: WorkspaceCommandSelectOption[];
  workspaceId: string | null;
  onWorkspaceChange: (workspaceId: string) => void;
  sessionName: string;
  onSessionNameChange: (next: string) => void;
  /** Resolved from the workspace payload by the dialog — the single source of truth. */
  workspaceSelected: boolean;
  isSubmitting: boolean;
}

/**
 * Simple tier of the session launcher: who runs, and where.
 *
 * Every field here is required for a launch, so nothing in this section may move
 * behind Advanced (`MODAL-STANDARD.md` § Hosts).
 */
function SessionCreateSimpleSection({
  agents,
  selectedAgentName,
  onAgentChange,
  workspaces,
  workspaceId,
  onWorkspaceChange,
  sessionName,
  onSessionNameChange,
  workspaceSelected,
  isSubmitting,
}: SessionCreateSimpleSectionProps) {
  const trimmedSelectedAgentName = selectedAgentName.trim();
  const activeAgent = workspaceSelected
    ? agents.find(agent => agent.name === trimmedSelectedAgentName)
    : undefined;
  const activeAgentProvider = resolveAgentRuntimeValue(activeAgent).provider;
  const hasAgents = agents.length > 0;
  const agentPlaceholder = !workspaceSelected
    ? "Select a workspace first"
    : hasAgents
      ? "Select an agent"
      : "No agents available";

  return (
    <FormSection
      description="Both choices come from runtime-backed catalogs."
      title="Who runs, and where"
    >
      <div className="flex flex-col gap-4">
        <Field>
          <FieldContent>
            <FieldLabel htmlFor="session-create-agent">
              Agent
              <RequiredMark />
            </FieldLabel>
            <FieldDescription>
              The agent owns the default prompt, tools, and provider for this session.
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
          {activeAgent && activeAgentProvider.length > 0 ? (
            <div
              className="mt-1 flex items-center gap-1.5 text-form-hint text-subtle"
              data-testid="session-create-agent-default"
            >
              <AgentIcon className="size-3 text-subtle" provider={activeAgentProvider} />
              <span>Effective provider: {activeAgentProvider}</span>
            </div>
          ) : null}
        </Field>

        <Field>
          <FieldContent>
            <FieldTitle id="session-create-workspace-label">
              Workspace
              <RequiredMark />
            </FieldTitle>
            <FieldDescription>The session reads and writes inside this root.</FieldDescription>
          </FieldContent>
          <div className="rounded-md border border-line bg-elevated">
            <WorkspaceCommandSelect
              disabled={isSubmitting}
              onChange={onWorkspaceChange}
              triggerTestId="session-create-workspace-select"
              value={workspaceId}
              workspaces={workspaces}
            />
          </div>
        </Field>

        <Field>
          <FieldContent>
            <FieldLabel htmlFor="session-create-name">Session name</FieldLabel>
            <FieldDescription>Optional — shown in the sidebar and session lists.</FieldDescription>
          </FieldContent>
          <Input
            data-testid="session-create-name-input"
            disabled={isSubmitting}
            id="session-create-name"
            onChange={event => onSessionNameChange(event.target.value)}
            placeholder="Investigate checkout latency"
            value={sessionName}
          />
        </Field>
      </div>
    </FormSection>
  );
}

export { SessionCreateSimpleSection };
