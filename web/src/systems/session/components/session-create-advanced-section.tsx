import {
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
  FormSection,
  Input,
  RequiredMark,
} from "@compozy/ui";

import type { NetworkParticipationDraft } from "@/lib/network-participation";
import { NetworkParticipationFields } from "@/systems/network";
import { WorkspaceCommandSelect, type WorkspaceCommandSelectOption } from "@/systems/workspace";

interface SessionCreateAdvancedSectionProps {
  networkParticipation: NetworkParticipationDraft;
  workspaces: WorkspaceCommandSelectOption[];
  workspaceId: string | null;
  userHomeDir: string | undefined;
  sessionName: string;
  workspacePath: string;
  onWorkspaceChange: (workspaceId: string) => void;
  onSessionNameChange: (next: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  onWorkspacePathChange: (next: string) => void;
  isSubmitting: boolean;
}

function SessionCreateAdvancedSection({
  networkParticipation,
  workspaces,
  workspaceId,
  userHomeDir,
  sessionName,
  workspacePath,
  onWorkspaceChange,
  onSessionNameChange,
  onNetworkParticipationChange,
  onWorkspacePathChange,
  isSubmitting,
}: SessionCreateAdvancedSectionProps) {
  return (
    <FormSection
      description="Choose where this session works and add optional session details."
      title="Session details"
    >
      <div className="flex flex-col gap-4">
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
              ariaLabelledBy="session-create-workspace-label"
              disabled={isSubmitting}
              onChange={onWorkspaceChange}
              triggerTestId="session-create-workspace-select"
              userHomeDir={userHomeDir}
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

        <Field>
          <FieldContent>
            <FieldLabel htmlFor="session-create-workspace-path">Working path</FieldLabel>
            <FieldDescription>
              Optional — use a relative subdirectory of the selected workspace.
            </FieldDescription>
          </FieldContent>
          <Input
            className="font-mono"
            data-testid="session-create-workspace-path-input"
            disabled={isSubmitting}
            id="session-create-workspace-path"
            onChange={event => onWorkspacePathChange(event.target.value)}
            placeholder="services/checkout"
            value={workspacePath}
          />
        </Field>

        <NetworkParticipationFields
          allowedStrategies={["named"]}
          disabled={isSubmitting}
          onChange={onNetworkParticipationChange}
          testIdPrefix="session-create-participation"
          value={networkParticipation}
        />
      </div>
    </FormSection>
  );
}

export { SessionCreateAdvancedSection };
