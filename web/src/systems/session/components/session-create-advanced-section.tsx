import {
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  FieldTitle,
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
  onWorkspaceChange: (workspaceId: string) => void;
  onSessionNameChange: (next: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  isSubmitting: boolean;
}

function SessionCreateAdvancedSection({
  networkParticipation,
  workspaces,
  workspaceId,
  userHomeDir,
  sessionName,
  onWorkspaceChange,
  onSessionNameChange,
  onNetworkParticipationChange,
  isSubmitting,
}: SessionCreateAdvancedSectionProps) {
  return (
    <>
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

      <NetworkParticipationFields
        allowedStrategies={["named"]}
        disabled={isSubmitting}
        onChange={onNetworkParticipationChange}
        testIdPrefix="session-create-participation"
        value={networkParticipation}
      />
    </>
  );
}

export { SessionCreateAdvancedSection };
