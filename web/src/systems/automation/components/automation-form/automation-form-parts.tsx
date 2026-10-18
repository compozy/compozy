import { Check, Eye } from "lucide-react";

import {
  Alert,
  AlertDescription,
  Button,
  EntityDialogFooter,
  Field,
  FieldError,
  FieldLabel,
  Input,
} from "@compozy/ui";

import { AutomationFormSection } from "./automation-form-section";
import { ProfileDestinationChip } from "@/systems/profiles";

/** 01 Name, with the daemon's name conflict on the field. */
export function AutomationNameSection({
  error,
  name,
  onName,
}: {
  error: string | null;
  name: string;
  onName: (name: string) => void;
}) {
  const invalid = error ? true : undefined;
  return (
    <AutomationFormSection
      data-testid="automation-form-name"
      description="How you'll find it in the list"
      number={1}
      title="Name"
    >
      <Field data-invalid={invalid}>
        <FieldLabel className="sr-only" htmlFor="automation-name">
          Automation name
        </FieldLabel>
        <Input
          aria-invalid={invalid}
          className="font-mono"
          data-testid="automation-name-input"
          id="automation-name"
          onChange={event => onName(event.target.value)}
          placeholder="morning-digest"
          value={name}
        />
        {error ? <FieldError>{error}</FieldError> : null}
      </Field>
    </AutomationFormSection>
  );
}

/** A save error that isn't about one field. */
export function AutomationFormErrorAlert({ error }: { error: string | null | undefined }) {
  if (!error) return null;
  return (
    <div className="shrink-0 px-5 pb-4">
      <Alert data-testid="automation-form-error" role="alert" variant="danger">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    </div>
  );
}

function primaryLabel(isPending: boolean, mode: "create" | "edit"): string {
  if (isPending) return "Saving…";
  return mode === "create" ? "Create automation" : "Save changes";
}

interface AutomationFormFooterProps {
  destination: string | null;
  isPending: boolean;
  mode: "create" | "edit";
  onCancel: () => void;
  onTogglePreview: () => void;
  previewing: boolean;
  profileDestination?: string | null;
  ready: boolean;
}

/** Production footer order: preview toggle · destination statement · Cancel · primary. */
export function AutomationFormFooter({
  destination,
  isPending,
  mode,
  onCancel,
  onTogglePreview,
  previewing,
  profileDestination,
  ready,
}: AutomationFormFooterProps) {
  const creating = mode === "create";
  return (
    <EntityDialogFooter
      hint={
        <AutomationDestinationNote
          destination={destination}
          mode={mode}
          profileDestination={creating ? profileDestination : null}
        />
      }
      isSaving={isPending}
      leading={
        <Button
          aria-pressed={previewing}
          data-testid="automation-preview-toggle"
          onClick={onTogglePreview}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Eye aria-hidden="true" className="size-3.5" />
          {previewing ? "Back to form" : "Show preview"}
        </Button>
      }
      onCancel={onCancel}
      primaryDisabled={!ready}
      primaryIcon={creating ? Check : undefined}
      primaryLabel={primaryLabel(isPending, mode)}
      primaryTestId="automation-form-submit"
      primaryType="submit"
    />
  );
}

/** `Creates in checkout-api.` · `Creates a Global automation.` · `Saves to checkout-api.` */
function AutomationDestinationNote({
  destination,
  mode,
  profileDestination,
}: {
  destination: string | null;
  mode: "create" | "edit";
  profileDestination?: string | null;
}) {
  const place = <span className="font-medium text-fg">{destination ?? "Global"}</span>;
  const text =
    mode === "edit" ? (
      <>Saves to {place}.</>
    ) : destination === null ? (
      <>Creates a {place} automation.</>
    ) : (
      <>Creates in {place}.</>
    );
  return (
    <span className="inline-flex max-w-full flex-wrap items-center gap-1.5">
      <span className="text-form-hint text-muted" data-testid="automation-destination">
        {text}
      </span>
      {profileDestination ? <ProfileDestinationChip profile={profileDestination} /> : null}
    </span>
  );
}
