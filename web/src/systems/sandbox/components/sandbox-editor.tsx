import { useState } from "react";
import { Boxes } from "lucide-react";

import { EntityModeToolbar, type EntityMode } from "@agh/ui";

import { SettingsEditorDialog, SettingsSourceBadge } from "@/systems/settings";

import {
  sandboxEnvErrors,
  sandboxSecretEnvErrors,
  type SandboxDraft,
} from "../lib/sandbox-profile-draft";
import type { SandboxEditorState } from "../hooks/use-sandbox-page";
import { SandboxEditorAdvancedSection } from "./sandbox-editor-advanced-section";
import { SandboxEditorSimpleSection } from "./sandbox-editor-simple-section";

export interface SandboxEditorProps {
  editor: SandboxEditorState;
  isValid: boolean;
  isSaving: boolean;
  error: string | null;
  warnings?: string[];
  existingNames: string[];
  initialTier?: EntityMode;
  onChange: (updater: (draft: SandboxDraft) => SandboxDraft) => void;
  onClose: () => void;
  onSave: () => void;
}

/**
 * Sandbox profile editor on the shared settings shell.
 *
 * Simple names the profile and picks where it runs; Advanced carries lifecycle,
 * environment, network policy, and — only for the Daytona backend — the cloud
 * workspace parameters.
 */
export function SandboxEditor({
  editor,
  isValid,
  isSaving,
  error,
  warnings,
  existingNames,
  initialTier = "simple",
  onChange,
  onClose,
  onSave,
}: SandboxEditorProps) {
  const [tier, setTier] = useState<EntityMode>(initialTier);
  const open = editor.mode !== "closed";
  if (!open) return null;

  const isCreate = editor.mode === "create";
  const draft = editor.draft;
  const entry = editor.mode === "edit" ? editor.entry : null;

  const lowerName = draft.name.trim().toLowerCase();
  const nameConflict =
    isCreate &&
    lowerName.length > 0 &&
    existingNames.some(existing => existing.toLowerCase() === lowerName);
  // A plaintext secret must never reach the replace PUT: the daemon stores
  // references only, so an unreferenced row blocks the save outright.
  const envErrors = sandboxEnvErrors(draft);
  const envError = Object.values(envErrors)[0] ?? null;
  const secretEnvErrors = sandboxSecretEnvErrors(draft);
  const secretEnvError = Object.values(secretEnvErrors)[0] ?? null;
  const nameError = nameConflict ? `A sandbox named "${draft.name}" already exists.` : null;

  return (
    <SettingsEditorDialog
      canSave={isValid && !nameConflict && envError === null && secretEnvError === null}
      description={
        isCreate
          ? "A profile decides where sessions execute — on this machine or in an isolated cloud workspace — and what they may reach."
          : "Update backend behavior and rotate write-only references. Sessions pick up changes on their next launch."
      }
      error={
        error ??
        nameError ??
        // Simple cannot render the offending row, so the blocked save states its cause here.
        (tier === "simple" ? (envError ?? secretEnvError) : null)
      }
      eyebrow="System · Sandbox"
      hint="Profiles are referenced by workspaces; replacing one changes every session that binds it."
      icon={Boxes}
      isSaving={isSaving}
      warnings={warnings}
      metadata={
        entry ? (
          <SettingsSourceBadge
            data-testid="sandbox-editor-source"
            shadowed={entry.source_metadata.shadowed_sources ?? []}
            source={entry.source_metadata.effective_source}
          />
        ) : null
      }
      mode={isCreate ? "create" : "edit"}
      onOpenChange={next => {
        if (!next) onClose();
      }}
      onSave={onSave}
      open={open}
      saveLabel={isCreate ? "Create profile" : "Save changes"}
      size="md"
      slug="sandbox"
      title={
        isCreate
          ? "Create sandbox profile"
          : `Edit ${editor.mode === "edit" ? editor.name : "sandbox profile"}`
      }
      toolbar={
        <EntityModeToolbar
          mode={tier}
          onModeChange={setTier}
          testIdPrefix="sandbox-editor"
          trailing={
            entry && entry.workspace_usage_count > 0 ? (
              <span
                className="font-mono text-form-label text-muted"
                data-testid="sandbox-editor-usage"
              >
                {entry.workspace_usage_count}{" "}
                {entry.workspace_usage_count === 1 ? "workspace" : "workspaces"}
              </span>
            ) : null
          }
        />
      }
    >
      <div className="flex flex-col">
        <SandboxEditorSimpleSection
          draft={draft}
          isCreate={isCreate}
          nameError={nameError}
          onChange={onChange}
          workspaceUsage={entry?.workspace_usage_count}
        />
        {tier === "advanced" ? (
          <SandboxEditorAdvancedSection
            draft={draft}
            envErrors={envErrors}
            onChange={onChange}
            secretEnvErrors={secretEnvErrors}
          />
        ) : null}
      </div>
    </SettingsEditorDialog>
  );
}
