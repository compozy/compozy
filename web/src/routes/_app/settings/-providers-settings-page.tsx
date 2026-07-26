import { AlertCircle, Check, Database, Plus, Trash2, X } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertAction,
  AlertDescription,
  Button,
  ConfirmDialog,
  Empty,
  Spinner,
} from "@agh/ui";

import { useCreateProviderFocusRestore } from "@/hooks/routes/use-create-provider-focus-restore";
import {
  useSettingsProvidersPage,
  type ProviderLastAction,
} from "@/systems/settings/hooks/use-settings-providers-page";
import {
  ProviderCard,
  ProviderDetailDialog,
  ProviderRow,
  ProvidersToolbar,
  SettingsPageFrame,
  useSettingsTopbar,
  type ProvidersViewMode,
  type SettingsProviderEntry,
} from "@/systems/settings";

export function ProvidersSettingsPage() {
  const page = useSettingsProvidersPage();
  const [view, setView] = useState<ProvidersViewMode>("cards");
  const inspectorOpen = page.inspector.mode !== "closed";
  const createProviderButtonRef = useCreateProviderFocusRestore(page.inspector.mode);
  useSettingsTopbar("providers", {
    actions:
      !page.isLoading && !page.error && page.envelope ? (
        <Button
          data-testid="settings-page-providers-create"
          onClick={page.openCreate}
          ref={createProviderButtonRef}
          size="sm"
          type="button"
        >
          <Plus aria-hidden="true" className="size-3" />
          New provider
        </Button>
      ) : undefined,
  });

  if (page.isLoading) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-providers-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-providers-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">{page.error?.message ?? "Failed to load providers"}</p>
        </div>
      </div>
    );
  }

  const inspectorEntry =
    page.inspector.mode === "inspect" || page.inspector.mode === "edit"
      ? page.inspector.entry
      : null;
  const inspectorDraft =
    page.inspector.mode === "edit" || page.inspector.mode === "create"
      ? page.inspector.draft
      : null;
  const readyCount = page.counts.installed;
  const setupCount = page.counts.needsSetup;
  const missingCount = page.counts.binaryMissing;

  return (
    <SettingsPageFrame
      description="The agent CLIs and model providers your sessions run on. Each provider is saved on its own."
      meta={[
        {
          key: "ready",
          content: <span data-testid="settings-page-providers-ready">{readyCount} ready</span>,
        },
        {
          key: "setup",
          content: (
            <span data-testid="settings-page-providers-needs-setup">{setupCount} needs setup</span>
          ),
        },
        {
          key: "missing",
          content: (
            <span data-testid="settings-page-providers-missing">{missingCount} not installed</span>
          ),
        },
      ]}
      restart={page.restart}
      slug="providers"
      width="wide"
    >
      {page.lastAction ? (
        <LastActionAlert action={page.lastAction} onDismiss={page.dismissLastAction} />
      ) : null}

      {page.providers.length === 0 ? (
        <Empty
          data-testid="settings-page-providers-empty"
          description='Use "New provider" to add an overlay entry to your config.'
          icon={Database}
          title="No providers configured"
        />
      ) : (
        <>
          <ProvidersToolbar
            nameQuery={page.filters.nameQuery}
            onNameQueryChange={page.setNameQuery}
            onStatusChange={page.setStatusFilter}
            onViewChange={setView}
            statusFilter={page.filters.statusFilter}
            view={view}
          />
          {page.filteredProviders.length === 0 ? (
            <p
              className="rounded-lg border border-line bg-canvas-soft px-4 py-4 text-small-body text-muted"
              data-testid="settings-page-providers-empty-filtered"
            >
              No providers match. Clear the search or the status filter.
            </p>
          ) : view === "cards" ? (
            <section
              className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
              data-testid="settings-page-providers-list"
            >
              {page.filteredProviders.map(provider => (
                <ProviderCard key={provider.name} onOpen={page.openInspect} provider={provider} />
              ))}
            </section>
          ) : (
            <section
              className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
              data-testid="settings-page-providers-list"
            >
              {page.filteredProviders.map(provider => (
                <ProviderRow key={provider.name} onOpen={page.openInspect} provider={provider} />
              ))}
            </section>
          )}
        </>
      )}

      <ProviderDetailDialog
        open={inspectorOpen}
        mode={page.inspector.mode === "closed" ? "inspect" : page.inspector.mode}
        entry={inspectorEntry}
        draft={inspectorDraft}
        error={page.inspectorError}
        warnings={page.inspectorWarnings}
        canSave={page.inspectorIsValid}
        isSaving={page.inspectorIsSaving}
        isDeleting={page.deleteIsPending}
        onOpenChange={next => {
          if (!next) page.closeInspector();
        }}
        onDraftChange={page.updateDraft}
        onSwitchToEdit={page.switchToEdit}
        onCancelEdit={page.cancelEdit}
        onSave={page.saveInspector}
        onRequestDelete={() => {
          if (inspectorEntry) page.openDelete(inspectorEntry);
        }}
        onRefreshCatalog={() => undefined}
      />

      <ProviderDeleteDialog
        target={page.deleteTarget.mode === "open" ? page.deleteTarget.entry : null}
        error={page.deleteError}
        isDeleting={page.deleteIsPending}
        onClose={page.closeDelete}
        onConfirm={page.confirmDelete}
      />
    </SettingsPageFrame>
  );
}

function ProviderDeleteDialog({
  target,
  error,
  isDeleting,
  onClose,
  onConfirm,
}: {
  target: SettingsProviderEntry | null;
  error: string | null;
  isDeleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const open = Boolean(target);
  const fallback = target?.fallback ?? null;

  return (
    <ConfirmDialog
      open={open}
      title={target ? `Delete provider "${target.name}"?` : "Delete provider"}
      description={
        target
          ? "Removing the overlay keeps the provider config in other overlays or builtin definitions, if any."
          : null
      }
      note={
        fallback ? (
          <div className="flex flex-col gap-1" data-testid="settings-providers-delete-builtin">
            <span className="font-medium">Builtin provider will be revealed</span>
            <span>
              After delete, the effective provider falls back to the builtin definition shipped with
              the daemon. The provider stays available with its shipped defaults.
            </span>
          </div>
        ) : null
      }
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel="Delete overlay"
      confirmIcon={Trash2}
      contentProps={{ "data-testid": "settings-providers-delete" }}
      titleProps={{ "data-testid": "settings-providers-delete-title" }}
      noteProps={{ "data-testid": "settings-providers-delete-fallback" }}
      errorProps={{ "data-testid": "settings-providers-delete-error" }}
      cancelButtonProps={{
        "data-testid": "settings-providers-delete-cancel",
        disabled: isDeleting,
      }}
      confirmButtonProps={{
        "data-testid": "settings-providers-delete-confirm",
      }}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    />
  );
}

function LastActionAlert({
  action,
  onDismiss,
}: {
  action: ProviderLastAction;
  onDismiss: () => void;
}) {
  const isSaved = action.kind === "saved";
  const restartBadge = action.result.restart_required
    ? "restart required to apply"
    : "applied immediately";

  const message = isSaved
    ? `Saved provider "${action.name}" · ${restartBadge}.`
    : action.hadFallback
      ? `Deleted overlay for "${action.name}" · builtin fallback now effective · ${restartBadge}.`
      : `Deleted overlay for "${action.name}" · ${restartBadge}.`;

  return (
    <Alert
      variant={isSaved ? "success" : "info"}
      role="status"
      data-testid="settings-page-providers-action-result"
      data-kind={action.kind}
    >
      <Check aria-hidden="true" className="size-3" />
      <AlertDescription className="text-xs">{message}</AlertDescription>
      <AlertAction>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onDismiss}
          data-testid="settings-page-providers-action-result-dismiss"
        >
          <X aria-hidden="true" className="size-3" />
        </Button>
      </AlertAction>
    </Alert>
  );
}
