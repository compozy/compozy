import { Pencil, Save, Settings2, Trash2 } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  LaneTabs,
  Pill,
  runViewTransition,
  viewTransitionName,
  type EntityMode,
} from "@compozy/ui";

import { getProviderStateView, type ProviderStateView } from "../lib/provider-state";
import type { ProviderDraft, SettingsProviderEntry } from "../types";
import { ProviderEditForm } from "./provider-edit-form";
import { ProviderInspectView } from "./provider-inspect-view";
import { SettingsSourceBadge } from "./settings-source-badge";

type DetailMode = "inspect" | "edit" | "create";

type DetailTab = "overview" | "configure";

export interface ProviderDetailDialogProps {
  open: boolean;
  mode: DetailMode;
  entry: SettingsProviderEntry | null;
  draft: ProviderDraft | null;
  error: string | null;
  warnings: string[] | undefined;
  canSave: boolean;
  isSaving: boolean;
  isDeleting: boolean;
  onOpenChange: (open: boolean) => void;
  onDraftChange: (updater: (draft: ProviderDraft) => ProviderDraft) => void;
  onSwitchToEdit: () => void;
  onCancelEdit: () => void;
  onSave: () => void;
  onRequestDelete: () => void;
  onRefreshCatalog: () => void;
}

/**
 * Provider detail on the shared entity-dialog shell (decision D1(b)): the
 * production dialog keeps inspect, edit, and create in one surface, and takes
 * the designed provider-sheet *body grammar* — `SettingsFieldRow` rows, auth
 * ownership cards, write-only credential slots — rather than the 576px sheet
 * host the artboards were drawn in.
 */
export function ProviderDetailDialog(props: ProviderDetailDialogProps) {
  const {
    open,
    mode,
    entry,
    draft,
    error,
    warnings,
    canSave,
    isSaving,
    isDeleting,
    onOpenChange,
    onDraftChange,
    onSwitchToEdit,
    onCancelEdit,
    onSave,
    onRequestDelete,
    onRefreshCatalog,
  } = props;
  const provider = entry;
  // The host keeps this dialog mounted between openings, so the disclosure tier
  // is reset whenever it switches entity or mode — a create must open on the
  // common path even if the last edit ended in Advanced.
  // Keyed by the entry, never by the draft: a create draft's name changes on
  // every keystroke and would reset the tier mid-typing.
  const surfaceKey = `${mode}:${entry?.name ?? "new"}`;
  const [tier, setTier] = useState<EntityMode>("simple");
  const [tierSurface, setTierSurface] = useState(surfaceKey);
  if (tierSurface !== surfaceKey) {
    setTierSurface(surfaceKey);
    setTier("simple");
  }

  const state = provider ? getProviderStateView(provider) : null;
  const isEditing = mode === "edit" || mode === "create";
  const isCreate = mode === "create";
  const activeTab: DetailTab = isEditing ? "configure" : "overview";
  const deletable = Boolean(
    provider && provider.source_metadata.effective_source.kind !== "builtin-provider"
  );

  // Overview and Configure differ a lot in height; a view transition morphs the
  // body instead of letting the dialog jump.
  const switchToEdit = () => void runViewTransition(onSwitchToEdit);
  const cancelEdit = () => void runViewTransition(onCancelEdit);
  const handleTabChange = (next: DetailTab) => {
    if (next === "configure" && mode === "inspect") switchToEdit();
    if (next === "overview" && mode === "edit") cancelEdit();
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange} disablePointerDismissal={false}>
      <DialogContent
        className={`text-fg grid-rows-[auto_minmax(0,1fr)_auto] ${dialogShellClass("md")}`}
        data-mode={mode}
        data-testid="provider-detail-dialog"
        showCloseButton={false}
        unframed
      >
        <div className="flex flex-col">
          <EntityDialogHeader
            className="border-b-0 pb-3"
            description={
              isCreate ? (
                "Connect an agent app to CompozyOS."
              ) : provider ? (
                <ProviderHeaderStatus isDefault={provider.default} state={state} />
              ) : undefined
            }
            // Create/edit titles already name the entity; only the bare-name inspect view needs the kind.
            eyebrow={mode === "inspect" ? "Provider" : undefined}
            icon={Settings2}
            onClose={isSaving ? undefined : () => onOpenChange(false)}
            title={
              <span data-testid="provider-detail-title">{headerTitle(mode, provider, draft)}</span>
            }
          />

          {isCreate ? null : (
            <div className="border-b border-line bg-canvas-soft px-5 pb-2">
              <LaneTabs<DetailTab>
                ariaLabel="Provider detail sections"
                items={[
                  { value: "overview", label: "Overview", testId: "provider-detail-tab-overview" },
                  {
                    value: "configure",
                    label: "Configure",
                    testId: "provider-detail-tab-configure",
                  },
                ]}
                onChange={handleTabChange}
                value={activeTab}
              />
            </div>
          )}

          {isEditing ? (
            <EntityModeToolbar
              mode={tier}
              onModeChange={setTier}
              testIdPrefix="settings-providers-editor"
              trailing={
                provider && tier === "advanced" ? (
                  <SettingsSourceBadge
                    data-testid="settings-providers-editor-source"
                    shadowed={provider.source_metadata.shadowed_sources ?? []}
                    source={provider.source_metadata.effective_source}
                  />
                ) : null
              }
            />
          ) : null}
        </div>

        <EntityDialogBody
          className="flex flex-col"
          data-testid="provider-detail-body"
          style={{ viewTransitionName: viewTransitionName("provider-detail-body") }}
        >
          {error ? (
            <Alert data-testid="provider-detail-error" variant="danger">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : null}
          {!error && warnings && warnings.length > 0 ? (
            <Alert data-testid="provider-detail-warnings" variant="warning">
              <AlertDescription>
                <ul className="flex flex-col gap-1">
                  {warnings.map(warning => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          ) : null}

          {isEditing ? (
            draft ? (
              <ProviderEditForm
                draft={draft}
                entry={provider}
                mode={isCreate ? "create" : "edit"}
                onChange={onDraftChange}
                tier={tier}
              />
            ) : null
          ) : provider ? (
            <ProviderInspectView
              onAction={switchToEdit}
              onRefreshCatalog={onRefreshCatalog}
              provider={provider}
            />
          ) : null}
        </EntityDialogBody>

        {isEditing ? (
          <EntityDialogFooter
            cancelDisabled={isSaving}
            cancelTestId="provider-detail-cancel"
            hint={
              isCreate
                ? "Key fields depend on who handles sign-in."
                : "Saved keys stay as they are unless you replace them."
            }
            isSaving={isSaving}
            onCancel={isCreate ? () => onOpenChange(false) : cancelEdit}
            onPrimary={onSave}
            primaryDisabled={!canSave}
            primaryIcon={Save}
            primaryLabel={isSaving ? "Saving…" : isCreate ? "Create provider" : "Save provider"}
            primaryTestId="provider-detail-save"
          />
        ) : (
          <EntityDialogFooter
            cancelLabel="Close"
            cancelTestId="provider-detail-close"
            hint={
              deletable ? (
                <Button
                  data-testid="provider-detail-delete"
                  disabled={isDeleting}
                  onClick={onRequestDelete}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  <Trash2 aria-hidden="true" className="size-3" />
                  {provider?.fallback ? "Reset to default" : "Delete provider"}
                </Button>
              ) : undefined
            }
            onCancel={() => onOpenChange(false)}
            onPrimary={switchToEdit}
            primaryDisabled={isDeleting}
            primaryIcon={Pencil}
            primaryLabel="Edit settings"
            primaryTestId="provider-detail-edit"
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function headerTitle(
  mode: DetailMode,
  provider: SettingsProviderEntry | null,
  draft: ProviderDraft | null
): string {
  if (mode === "create") return "Create provider";
  const name = provider?.settings.display_name?.trim() || provider?.name || draft?.name || "";
  return mode === "edit" ? `Edit ${name}` : name;
}

function ProviderHeaderStatus({
  isDefault,
  state,
}: {
  isDefault: boolean;
  state: ProviderStateView | null;
}) {
  if (!isDefault && !state) return null;
  return (
    <span className="flex flex-wrap items-center gap-2" data-testid="provider-detail-status">
      {state ? (
        <Pill tone={state.label === "installed" ? "neutral" : state.tone}>
          <Pill.Dot tone={state.label === "installed" ? "success" : state.tone} />
          {state.display}
        </Pill>
      ) : null}
      {isDefault ? <Pill tone="accent">Default</Pill> : null}
    </span>
  );
}
