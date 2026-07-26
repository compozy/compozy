import { Eyebrow, FormSection, ImmutableIdentity, Input, RequiredMark } from "@agh/ui";

import type { ProviderDraft } from "../types";
import type { ProviderDraftChange } from "./provider-edit-form";
import { ModalSettingsFieldRow } from "./settings-field-row";

interface ProviderGeneralFieldsProps {
  mode: "create" | "edit";
  draft: ProviderDraft;
  onChange: ProviderDraftChange;
}

/**
 * Provider basics — the minimum needed to launch an ACP subprocess.
 *
 * On edit the name renders as `ImmutableIdentity`: `PutSettingsProvider` keys
 * the overlay by its path name, so renaming is a create, and a disabled input
 * is forbidden as a data display (`MODAL-STANDARD.md` § Component grammar).
 */
export function ProviderGeneralFields({ mode, draft, onChange }: ProviderGeneralFieldsProps) {
  const isCreate = mode === "create";

  return (
    <FormSection
      data-testid="settings-providers-editor-basics"
      description="The minimum needed to launch an ACP subprocess."
      title="Provider basics"
    >
      {isCreate ? (
        <ModalSettingsFieldRow
          control={
            <Input
              autoFocus
              className="w-56 font-mono"
              data-testid="settings-providers-editor-name-input"
              onChange={event => onChange(current => ({ ...current, name: event.target.value }))}
              placeholder="e.g. claude"
              value={draft.name}
            />
          }
          data-testid="settings-providers-editor-name"
          description="Lower-case identifier used in agent frontmatter and CLI flags."
          label={
            <>
              Name
              <RequiredMark />
            </>
          }
        />
      ) : (
        <ImmutableIdentity
          data-testid="settings-providers-editor-identity"
          hint="Provider identity is immutable — create a new provider to rename it."
          rows={[{ label: "Provider", mono: true, value: draft.name }]}
        />
      )}

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-56"
            data-testid="settings-providers-editor-display-name-input"
            onChange={event =>
              onChange(current => ({ ...current, display_name: event.target.value }))
            }
            placeholder="OpenRouter"
            value={draft.display_name}
          />
        }
        data-testid="settings-providers-editor-display-name"
        description="Operator-facing label shown beside the provider id."
        label={
          <>
            Display name
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-providers-editor-command-input"
            onChange={event => onChange(current => ({ ...current, command: event.target.value }))}
            placeholder="npx @agentclientprotocol/claude-agent-acp@latest"
            value={draft.command}
          />
        }
        data-testid="settings-providers-editor-command"
        description={
          isCreate
            ? "Executable used to launch the ACP subprocess."
            : "Executable used to launch the ACP subprocess. Clearing it falls back to the builtin definition."
        }
        label={
          <>
            Command
            {isCreate ? <RequiredMark /> : null}
          </>
        }
      />
    </FormSection>
  );
}
