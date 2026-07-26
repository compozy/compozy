import { AlertCircle, KeyRound, Lock } from "lucide-react";

import {
  Alert,
  AlertAction,
  AlertDescription,
  Eyebrow,
  FormSection,
  Input,
  SecretField,
  Switch,
} from "@agh/ui";

import { ModalSettingsFieldRow, SettingsEditorDialog } from "@/systems/settings";

import type { VaultDraft, VaultEditorState } from "../hooks/use-vault-page";

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
  const refError =
    draft.ref.trim() && !draft.ref.trim().startsWith("vault:")
      ? "Vault refs must start with vault:."
      : null;

  return (
    <SettingsEditorDialog
      open
      mode="create"
      icon={KeyRound}
      eyebrow="System · Vault"
      size="sm"
      title="Add vault secret"
      slug="vault"
      description="Stores a write-only secret value and returns redacted metadata."
      hint={
        <>
          Bind it from providers, bridges, or sandboxes as{" "}
          <b className="font-medium text-muted">vault:&lt;reference&gt;</b>.
        </>
      }
      error={error ?? refError}
      canSave={canSave && !refError}
      isSaving={isSaving}
      saveLabel="Store secret"
      onSave={onSave}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <div className="flex flex-col">
        <FormSection
          help="Providers, bridges, and sandboxes bind this value by its reference, so the reference is the only part of a secret that stays readable."
          title="The reference"
        >
          <div className="flex flex-col gap-4.5">
            <ModalSettingsFieldRow
              label={
                <>
                  Secret reference
                  <Eyebrow className="ml-1.5 text-accent-strong">required</Eyebrow>
                </>
              }
              error={refError}
              data-testid="settings-vault-editor-ref"
              control={
                <Input
                  className="w-full font-mono"
                  value={draft.ref}
                  onChange={event => onChange(current => ({ ...current, ref: event.target.value }))}
                  placeholder="vault:sessions/sess_123/github-token"
                  data-testid="settings-vault-editor-ref-input"
                />
              }
            />
            {refExists ? (
              <Alert variant="warning" data-testid="settings-vault-editor-overwrite">
                <AlertCircle className="size-4" />
                <AlertDescription>
                  This reference already exists. Saving rotates its write-only value — everything
                  bound to it starts using the new secret, and the current value stays unreadable.
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
                    Confirm replacement
                  </label>
                </AlertAction>
              </Alert>
            ) : null}
            <ModalSettingsFieldRow
              label="Kind"
              help="A metadata label returned on public Vault surfaces. It groups secrets; it never affects how the value is stored."
              data-testid="settings-vault-editor-kind"
              control={
                <Input
                  className="w-48 font-mono"
                  value={draft.kind}
                  onChange={event =>
                    onChange(current => ({ ...current, kind: event.target.value }))
                  }
                  placeholder="api_key"
                  data-testid="settings-vault-editor-kind-input"
                />
              }
            />
          </div>
        </FormSection>

        <FormSection description="Written once, retrievable never." title="The value">
          <div className="flex flex-col gap-3">
            <SecretField
              id="settings-vault-editor-secret-value"
              label="Secret value"
              description="Write-only payload. The daemon never returns this value."
              placeholder="Paste the secret value"
              required
              saving={isSaving}
              testIdPrefix="settings-vault-editor-secret-value"
              value={draft.secretValue}
              onValueChange={next => onChange(current => ({ ...current, secretValue: next }))}
            />
            <Alert data-testid="settings-vault-editor-boundary">
              <Lock className="size-4" />
              <AlertDescription>
                Write-only boundary. Later edits rotate this value — they never display it. Only the
                reference, kind, and presence stay readable.
              </AlertDescription>
            </Alert>
          </div>
        </FormSection>
      </div>
    </SettingsEditorDialog>
  );
}
