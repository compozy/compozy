import { Field, FieldContent, FieldDescription, FieldLabel, FormSection, Input } from "@agh/ui";

import { NetworkParticipationFields, type NetworkParticipationDraft } from "@/systems/network";

interface SessionCreateAdvancedSectionProps {
  networkParticipation: NetworkParticipationDraft;
  workspacePath: string;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  onWorkspacePathChange: (next: string) => void;
  isSubmitting: boolean;
}

function SessionCreateAdvancedSection({
  networkParticipation,
  workspacePath,
  onNetworkParticipationChange,
  onWorkspacePathChange,
  isSubmitting,
}: SessionCreateAdvancedSectionProps) {
  return (
    <FormSection
      description="Override the agent defaults for this launch only."
      title="Launch overrides"
    >
      <div className="flex flex-col gap-4">
        <NetworkParticipationFields
          allowedStrategies={["named"]}
          disabled={isSubmitting}
          onChange={onNetworkParticipationChange}
          testIdPrefix="session-create-participation"
          value={networkParticipation}
        />

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
      </div>
    </FormSection>
  );
}

export { SessionCreateAdvancedSection };
