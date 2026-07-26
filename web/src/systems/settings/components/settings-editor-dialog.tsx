import { AlertCircle } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  dialogShellClass,
  type DialogShellSize,
} from "@agh/ui";

type EditorMode = "create" | "edit";

interface SettingsEditorDialogProps {
  open: boolean;
  mode: EditorMode;
  /** Entity glyph for the header icon well. */
  icon: LucideIcon;
  /** Domain path above the title, e.g. `System · Vault`. */
  eyebrow: string;
  /** Modal host size from the shared size map. */
  size: DialogShellSize;
  title: string;
  slug?: string;
  description?: ReactNode;
  metadata?: ReactNode;
  /** Consequence note rendered on the leading edge of the footer. */
  hint?: ReactNode;
  error?: string | null;
  warnings?: string[];
  canSave: boolean;
  isSaving: boolean;
  saveLabel?: string;
  onSave: () => void;
  onOpenChange: (open: boolean) => void;
  /**
   * Optional disclosure toolbar between the header and the body. Supply an
   * `EntityModeToolbar` when the surface has a Simple/Advanced tier; omit it and
   * the shell keeps its four-row template unchanged.
   */
  toolbar?: ReactNode;
  children: ReactNode;
}

/**
 * Shared create/edit shell for settings-owned entities (vault secrets, sandbox
 * profiles). Owns the shared header, size-aware dialog host, and footer so
 * each page supplies only its body and copy — no per-page shell forks.
 */
function SettingsEditorDialog({
  open,
  mode,
  icon,
  eyebrow,
  size,
  title,
  slug,
  description,
  metadata,
  hint,
  error,
  warnings,
  canSave,
  isSaving,
  saveLabel,
  onSave,
  onOpenChange,
  toolbar,
  children,
}: SettingsEditorDialogProps) {
  const computedSaveLabel = saveLabel ?? (mode === "create" ? "Create" : "Save changes");
  const testSlug = slug ?? "modal";
  const feedback = error || (warnings && warnings.length > 0);
  // The disclosure toolbar is its own row so the body keeps sole scroll ownership.
  const rowTemplate = toolbar
    ? "grid-rows-[auto_auto_minmax(0,1fr)_auto_auto]"
    : "grid-rows-[auto_minmax(0,1fr)_auto_auto]";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={`${rowTemplate} ${dialogShellClass(size)}`}
        unframed
        data-testid={`settings-${testSlug}-editor`}
        data-mode={mode}
      >
        <EntityDialogHeader
          description={
            description ? (
              <span data-testid={`settings-${testSlug}-editor-description`}>{description}</span>
            ) : undefined
          }
          eyebrow={eyebrow}
          icon={icon}
          title={<span data-testid={`settings-${testSlug}-editor-title`}>{title}</span>}
        />

        {toolbar}

        <EntityDialogBody data-testid={`settings-${testSlug}-editor-body`}>
          {metadata ? (
            <div data-testid={`settings-${testSlug}-editor-metadata`} className="mb-4">
              {metadata}
            </div>
          ) : null}
          {children}
        </EntityDialogBody>

        {feedback ? (
          <div
            className="flex flex-col gap-2 px-5 pb-4"
            data-testid={`settings-${testSlug}-editor-feedback`}
          >
            {error ? (
              <Alert variant="danger" data-testid={`settings-${testSlug}-editor-error`}>
                <AlertCircle className="mt-0.5 size-3 shrink-0" />
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            {!error && warnings && warnings.length > 0 ? (
              <Alert variant="warning" data-testid={`settings-${testSlug}-editor-warnings`}>
                <AlertCircle className="mt-0.5 size-3 shrink-0" />
                <AlertDescription>
                  <ul className="flex flex-col gap-1">
                    {warnings.map(warning => (
                      <li key={warning}>{warning}</li>
                    ))}
                  </ul>
                </AlertDescription>
              </Alert>
            ) : null}
          </div>
        ) : null}

        <EntityDialogFooter
          cancelDisabled={isSaving}
          cancelTestId={`settings-${testSlug}-editor-cancel`}
          hint={hint}
          hintTestId={`settings-${testSlug}-editor-hint`}
          isSaving={isSaving}
          onCancel={() => onOpenChange(false)}
          onPrimary={onSave}
          primaryDisabled={!canSave}
          primaryLabel={isSaving ? "Saving..." : computedSaveLabel}
          primaryTestId={`settings-${testSlug}-editor-save`}
        />
      </DialogContent>
    </Dialog>
  );
}

export { SettingsEditorDialog };
export type { EditorMode };
