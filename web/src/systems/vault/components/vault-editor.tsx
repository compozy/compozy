import { AlertCircle, ChevronRight, KeyRound } from "lucide-react";

import {
  Alert,
  AlertAction,
  AlertDescription,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Input,
  MonoId,
  RequiredMark,
  SecretField,
  Switch,
} from "@compozy/ui";

import { normalizeVaultRef, type VaultDraft, type VaultEditorState } from "../hooks/use-vault-page";
import { ModalSettingsFieldRow, SettingsEditorDialog } from "@/systems/settings";

interface VaultEditorProps {
  editor: VaultEditorState;
  isSaving: boolean;
  canSave: boolean;
  error: string | null;
  /** The normalized ref already exists, so saving rotates its stored value. */
  refExists: boolean;
  onChange: (updater: (draft: VaultDraft) => VaultDraft) => void;
  onClose: () => void;
  onSave: () => void;
}

export function VaultEditor({
  editor,
  isSaving,
  canSave,
  error,
  refExists,
  onChange,
  onClose,
  onSave,
}: VaultEditorProps) {
  if (editor.mode === "closed") {
    return null;
  }

  const draft = editor.draft;
  const savedAs = normalizeVaultRef(draft.ref);

  return (
    <SettingsEditorDialog
      open
      mode="create"
      icon={KeyRound}
      eyebrow="Vault"
      size="sm"
      title="New secret"
      slug="vault"
      description="Save an API key or token. Agents and extensions use it by name."
      hint="You won't be able to see this value again after saving."
      error={error}
      canSave={canSave}
      isSaving={isSaving}
      saveLabel="Save secret"
      onSave={onSave}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <div className="flex flex-col gap-4.5">
        <ModalSettingsFieldRow
          label={
            <>
              Name
              <RequiredMark />
            </>
          }
          description={
            savedAs ? (
              <span data-testid="settings-vault-editor-ref-preview">
                Saved as <MonoId preserveCase value={savedAs} />
              </span>
            ) : undefined
          }
          data-testid="settings-vault-editor-ref"
          control={
            <Input
              className="w-full font-mono"
              value={draft.ref}
              onChange={event => onChange(current => ({ ...current, ref: event.target.value }))}
              placeholder="providers/openai/api-key"
              data-testid="settings-vault-editor-ref-input"
            />
          }
        />
        {refExists ? (
          <Alert variant="warning" data-testid="settings-vault-editor-overwrite">
            <AlertCircle className="size-4" />
            <AlertDescription>
              A secret with this name already exists. Saving replaces its value everywhere it's
              used.
            </AlertDescription>
            <AlertAction>
              <label className="flex items-center gap-2 text-form-label text-muted">
                <Switch
                  checked={draft.overwriteConfirmed}
                  data-testid="settings-vault-editor-overwrite-confirm"
                  onCheckedChange={next =>
                    onChange(current => ({ ...current, overwriteConfirmed: next === true }))
                  }
                />
                Replace it
              </label>
            </AlertAction>
          </Alert>
        ) : null}
        <SecretField
          id="settings-vault-editor-secret-value"
          label="Secret value"
          placeholder="Paste the secret value"
          required
          saving={isSaving}
          testIdPrefix="settings-vault-editor-secret-value"
          value={draft.secretValue}
          onValueChange={next => onChange(current => ({ ...current, secretValue: next }))}
        />
        <Collapsible data-testid="settings-vault-editor-more">
          <CollapsibleTrigger className="group flex items-center gap-1 text-form-label text-muted hover:text-fg">
            <ChevronRight
              aria-hidden="true"
              className="size-3 transition-transform group-data-panel-open:rotate-90 motion-reduce:transition-none"
            />
            More options
          </CollapsibleTrigger>
          <CollapsibleContent className="pt-3">
            <ModalSettingsFieldRow
              label="Label (optional)"
              data-testid="settings-vault-editor-kind"
              control={
                <Input
                  className="w-48"
                  value={draft.kind}
                  onChange={event =>
                    onChange(current => ({ ...current, kind: event.target.value }))
                  }
                  placeholder="api_key"
                  data-testid="settings-vault-editor-kind-input"
                />
              }
            />
          </CollapsibleContent>
        </Collapsible>
      </div>
    </SettingsEditorDialog>
  );
}
