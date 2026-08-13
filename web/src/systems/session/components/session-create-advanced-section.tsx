import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  Spinner,
} from "@compozy/ui";

import type { NetworkParticipationDraft } from "@/lib/network-participation";

import { NetworkParticipationFields } from "@/systems/network";

import { SessionEnvironmentField } from "./session-environment-field";

interface SessionCreateAdvancedSectionProps {
  networkParticipation: NetworkParticipationDraft;
  sessionName: string;
  onSessionNameChange: (next: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  isSubmitting: boolean;
  /** Absent when the selected workspace is not git-backed — there is nothing to choose. */
  environment?: React.ComponentProps<typeof SessionEnvironmentField>;
  environmentListingState: "loading" | "ready" | "error" | "unsupported";
  environmentListingError?: string;
}

function SessionCreateAdvancedSection({
  networkParticipation,
  sessionName,
  onSessionNameChange,
  onNetworkParticipationChange,
  isSubmitting,
  environment,
  environmentListingState,
  environmentListingError,
}: SessionCreateAdvancedSectionProps) {
  return (
    <>
      {environment ? (
        <SessionEnvironmentField {...environment} />
      ) : environmentListingState === "loading" ? (
        <Field aria-busy="true">
          <FieldLabel>Environment</FieldLabel>
          <Spinner className="size-4" />
        </Field>
      ) : environmentListingState === "error" ? (
        <Field data-invalid="">
          <FieldLabel>Environment</FieldLabel>
          <FieldError>{environmentListingError || "Environments could not be loaded."}</FieldError>
        </Field>
      ) : null}

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
