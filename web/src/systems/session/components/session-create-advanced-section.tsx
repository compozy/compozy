import { Field, FieldContent, FieldDescription, FieldLabel, Input } from "@compozy/ui";

import type { NetworkParticipationDraft } from "@/lib/network-participation";

import { NetworkParticipationFields } from "@/systems/network";

interface SessionCreateAdvancedSectionProps {
  networkParticipation: NetworkParticipationDraft;
  sessionName: string;
  onSessionNameChange: (next: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  isSubmitting: boolean;
}

function SessionCreateAdvancedSection({
  networkParticipation,
  sessionName,
  onSessionNameChange,
  onNetworkParticipationChange,
  isSubmitting,
}: SessionCreateAdvancedSectionProps) {
  return (
    <>
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
