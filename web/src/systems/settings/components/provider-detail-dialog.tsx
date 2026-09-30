import { Pencil, Save, Settings2, Trash2 } from "lucide-react";
import type { ReactNode } from "react";

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

import { useProviderEditorTier } from "../hooks/use-provider-editor-tier";
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
    entry: provider,
    isSaving,
    onOpenChange,
    onSwitchToEdit,
    onCancelEdit,
  } = props;
  const [tier, setTier] = useProviderEditorTier(`${mode}:${provider?.name ?? "new"}`);
  const isEditing = mode === "edit" || mode === "create";

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
          <ProviderDetailHeader
            draft={props.draft}
            mode={mode}
            onClose={isSaving ? undefined : () => onOpenChange(false)}
            provider={provider}
          />
          {mode === "create" ? null : (
            <ProviderDetailTabs
              activeTab={isEditing ? "configure" : "overview"}
              onChange={handleTabChange}
            />
          )}
          {isEditing ? (
            <ProviderEditorToolbar provider={provider} tier={tier} onTierChange={setTier} />
          ) : null}
        </div>

        <EntityDialogBody
          className="flex flex-col"
          data-testid="provider-detail-body"
          style={{ viewTransitionName: viewTransitionName("provider-detail-body") }}
        >
          <ProviderDetailNotices error={props.error} warnings={props.warnings} />
          <ProviderDetailContent
            draft={props.draft}
            mode={mode}
            onDraftChange={props.onDraftChange}
            onRefreshCatalog={props.onRefreshCatalog}
            onSwitchToEdit={switchToEdit}
            provider={provider}
            tier={tier}
          />
        </EntityDialogBody>

        {isEditing ? (
          <ProviderEditFooter
            canSave={props.canSave}
            isCreate={mode === "create"}
            isSaving={isSaving}
            onCancel={mode === "create" ? () => onOpenChange(false) : cancelEdit}
            onSave={props.onSave}
          />
        ) : (
          <ProviderInspectFooter
            isDeleting={props.isDeleting}
            onClose={() => onOpenChange(false)}
            onEdit={switchToEdit}
            onRequestDelete={props.onRequestDelete}
            provider={provider}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function ProviderDetailHeader({
  mode,
  provider,
  draft,
  onClose,
}: {
  mode: DetailMode;
  provider: SettingsProviderEntry | null;
  draft: ProviderDraft | null;
  onClose: (() => void) | undefined;
}) {
  return (
    <EntityDialogHeader
      className="border-b-0 pb-3"
      description={headerDescription(mode, provider)}
      // Create/edit titles already name the entity; only the bare-name inspect view needs the kind.
      eyebrow={mode === "inspect" ? "Provider" : undefined}
      icon={Settings2}
      onClose={onClose}
      title={<span data-testid="provider-detail-title">{headerTitle(mode, provider, draft)}</span>}
    />
  );
}

function headerDescription(mode: DetailMode, provider: SettingsProviderEntry | null): ReactNode {
  if (mode === "create") return "Connect an agent app to CompozyOS.";
  if (!provider) return undefined;
  return (
    <ProviderHeaderStatus isDefault={provider.default} state={getProviderStateView(provider)} />
  );
}

const DETAIL_TABS = [
  { value: "overview", label: "Overview", testId: "provider-detail-tab-overview" },
  { value: "configure", label: "Configure", testId: "provider-detail-tab-configure" },
] satisfies { value: DetailTab; label: string; testId: string }[];

function ProviderDetailTabs({
  activeTab,
  onChange,
}: {
  activeTab: DetailTab;
  onChange: (next: DetailTab) => void;
}) {
  return (
    <div className="border-b border-line px-5 pb-2">
      <LaneTabs<DetailTab>
        ariaLabel="Provider detail sections"
        items={DETAIL_TABS}
        onChange={onChange}
        value={activeTab}
      />
    </div>
  );
}

function ProviderEditorToolbar({
  provider,
  tier,
  onTierChange,
}: {
  provider: SettingsProviderEntry | null;
  tier: EntityMode;
  onTierChange: (tier: EntityMode) => void;
}) {
  return (
    <EntityModeToolbar
      mode={tier}
      onModeChange={onTierChange}
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
  );
}

function ProviderDetailNotices({
  error,
  warnings,
}: {
  error: string | null;
  warnings: string[] | undefined;
}) {
  if (error) {
    return (
      <Alert data-testid="provider-detail-error" variant="danger">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    );
  }
  if (!warnings || warnings.length === 0) return null;
  return (
    <Alert data-testid="provider-detail-warnings" variant="warning">
      <AlertDescription>
        <ul className="flex flex-col gap-1">
          {warnings.map(warning => (
            <li key={warning}>{warning}</li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  );
}

function ProviderDetailContent({
  mode,
  provider,
  draft,
  tier,
  onDraftChange,
  onSwitchToEdit,
  onRefreshCatalog,
}: {
  mode: DetailMode;
  provider: SettingsProviderEntry | null;
  draft: ProviderDraft | null;
  tier: EntityMode;
  onDraftChange: ProviderDetailDialogProps["onDraftChange"];
  onSwitchToEdit: () => void;
  onRefreshCatalog: () => void;
}) {
  if (mode === "inspect") {
    if (!provider) return null;
    return (
      <ProviderInspectView
        onAction={onSwitchToEdit}
        onRefreshCatalog={onRefreshCatalog}
        provider={provider}
      />
    );
  }
  if (!draft) return null;
  return (
    <ProviderEditForm
      draft={draft}
      entry={provider}
      mode={mode}
      onChange={onDraftChange}
      tier={tier}
    />
  );
}

function ProviderEditFooter({
  isCreate,
  isSaving,
  canSave,
  onCancel,
  onSave,
}: {
  isCreate: boolean;
  isSaving: boolean;
  canSave: boolean;
  onCancel: () => void;
  onSave: () => void;
}) {
  const saveLabel = isCreate ? "Create provider" : "Save provider";
  return (
    <EntityDialogFooter
      cancelDisabled={isSaving}
      cancelTestId="provider-detail-cancel"
      hint={
        isCreate
          ? "Key fields depend on who handles sign-in."
          : "Saved keys stay as they are unless you replace them."
      }
      isSaving={isSaving}
      onCancel={onCancel}
      onPrimary={onSave}
      primaryDisabled={!canSave}
      primaryIcon={Save}
      primaryLabel={isSaving ? "Saving…" : saveLabel}
      primaryTestId="provider-detail-save"
    />
  );
}

function ProviderInspectFooter({
  provider,
  isDeleting,
  onClose,
  onEdit,
  onRequestDelete,
}: {
  provider: SettingsProviderEntry | null;
  isDeleting: boolean;
  onClose: () => void;
  onEdit: () => void;
  onRequestDelete: () => void;
}) {
  const deletable = Boolean(
    provider && provider.source_metadata.effective_source.kind !== "builtin-provider"
  );
  return (
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
      onCancel={onClose}
      onPrimary={onEdit}
      primaryDisabled={isDeleting}
      primaryIcon={Pencil}
      primaryLabel="Edit settings"
      primaryTestId="provider-detail-edit"
    />
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
