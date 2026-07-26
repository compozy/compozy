import { Cloud, MonitorSmartphone } from "lucide-react";

import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FormSection,
  ImmutableIdentity,
  Input,
  NativeSelect,
  NativeSelectOption,
  Pill,
  RadioCard,
  RequiredMark,
} from "@agh/ui";

import type { SandboxDraft } from "../lib/sandbox-profile-draft";

const SYNC_MODES: readonly { value: string; label: string }[] = [
  { value: "", label: "Provider default" },
  { value: "none", label: "None — no file sync" },
  { value: "session-bidirectional", label: "Session bidirectional — sync at start and stop" },
  { value: "turn-bidirectional", label: "Turn bidirectional — sync every turn" },
];

export interface SandboxEditorSimpleSectionProps {
  draft: SandboxDraft;
  isCreate: boolean;
  nameError?: string | null;
  workspaceUsage?: number;
  onChange: (updater: (draft: SandboxDraft) => SandboxDraft) => void;
}

/**
 * Simple tier: what the profile is called, where it runs, and how files move.
 *
 * A stored backend outside local/Daytona (an `e2b` profile, for instance) is
 * reported as-is rather than silently rewritten — this editor can only switch
 * to the two backends it can configure.
 */
export function SandboxEditorSimpleSection({
  draft,
  isCreate,
  nameError,
  workspaceUsage,
  onChange,
}: SandboxEditorSimpleSectionProps) {
  const backend = draft.backend.trim();
  const isUnlistedBackend = backend !== "local" && backend !== "daytona";

  return (
    <>
      <FormSection
        data-testid="sandbox-editor-profile"
        description="Safe defaults are enough for most agent sessions."
        title="The profile"
      >
        {isCreate ? (
          <Field data-invalid={Boolean(nameError)}>
            <FieldLabel htmlFor="sandbox-editor-name">
              Profile name
              <RequiredMark />
            </FieldLabel>
            <Input
              aria-invalid={Boolean(nameError)}
              className="font-mono"
              data-testid="sandbox-editor-name"
              id="sandbox-editor-name"
              onChange={event => onChange(current => ({ ...current, name: event.target.value }))}
              placeholder="isolated-dev"
              value={draft.name}
            />
            <FieldError data-testid="sandbox-editor-name-error">{nameError}</FieldError>
          </Field>
        ) : (
          <ImmutableIdentity
            hint={
              workspaceUsage
                ? `Profile identity is fixed on this route — assigned to ${workspaceUsage} ${
                    workspaceUsage === 1 ? "workspace" : "workspaces"
                  }.`
                : "Profile identity is fixed on this route — create a new profile to rename it."
            }
            rows={[{ label: "Profile", mono: true, value: draft.name }]}
          />
        )}
      </FormSection>

      <FormSection
        data-testid="sandbox-editor-backend-section"
        description={
          isCreate
            ? "The execution backend for sessions using this profile."
            : "Switching backend applies to new sessions only."
        }
        title="Where does it run?"
      >
        <div
          aria-label="Execution backend"
          className="grid gap-2 sm:grid-cols-2"
          data-testid="sandbox-editor-backend"
          role="radiogroup"
        >
          <RadioCard
            data-testid="sandbox-editor-backend-local"
            // The execution model stacks under the description: an inline badge
            // pushes the title off its own row at two-up.
            description={
              <>
                Runs as a host process. Fastest, shares this machine.
                <Pill className="mt-1.5 flex w-fit" size="xs">
                  Host process
                </Pill>
              </>
            }
            icon={MonitorSmartphone}
            onSelect={() => onChange(current => ({ ...current, backend: "local" }))}
            selected={backend === "local"}
            title="Local"
            titleClassName="min-w-0 flex-1 truncate"
          />
          <RadioCard
            data-testid="sandbox-editor-backend-daytona"
            description={
              <>
                Isolated cloud workspace with its own filesystem and network.
                <Pill className="mt-1.5 flex w-fit" size="xs">
                  Cloud workspace
                </Pill>
              </>
            }
            icon={Cloud}
            onSelect={() => onChange(current => ({ ...current, backend: "daytona" }))}
            selected={backend === "daytona"}
            title="Daytona"
            titleClassName="min-w-0 flex-1 truncate"
          />
        </div>

        {isUnlistedBackend ? (
          <p className="text-form-hint text-subtle" data-testid="sandbox-editor-backend-unlisted">
            This profile runs on <Pill mono>{backend}</Pill>, which this editor cannot configure.
            Choosing Local or Daytona replaces it.
          </p>
        ) : null}

        <Field>
          <FieldLabel htmlFor="sandbox-editor-sync-mode">Workspace sync</FieldLabel>
          <FieldDescription>How files move between the workspace and the sandbox.</FieldDescription>
          <NativeSelect
            data-testid="sandbox-editor-sync-mode"
            id="sandbox-editor-sync-mode"
            onChange={event => onChange(current => ({ ...current, sync_mode: event.target.value }))}
            value={draft.sync_mode}
          >
            {SYNC_MODES.map(option => (
              <NativeSelectOption key={option.value || "default"} value={option.value}>
                {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>
      </FormSection>
    </>
  );
}
