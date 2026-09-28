import { Field, FieldError, FieldLabel, Input, Spinner } from "@compozy/ui";

import { SessionEnvironmentField } from "./session-environment-field";

interface SessionCreateAdvancedSectionProps {
  sessionName: string;
  onSessionNameChange: (next: string) => void;
  isSubmitting: boolean;
  /** Absent when the selected workspace is not git-backed — there is nothing to choose. */
  environment?: React.ComponentProps<typeof SessionEnvironmentField>;
  environmentListingState: "loading" | "ready" | "error" | "unsupported";
  environmentListingError?: string;
}

function SessionCreateAdvancedSection({
  sessionName,
  onSessionNameChange,
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
        <FieldLabel htmlFor="session-create-name">Session name</FieldLabel>
        <Input
          data-testid="session-create-name-input"
          disabled={isSubmitting}
          id="session-create-name"
          onChange={event => onSessionNameChange(event.target.value)}
          placeholder="Investigate checkout latency"
          value={sessionName}
        />
      </Field>
    </>
  );
}

export { SessionCreateAdvancedSection };
