import { Check, Database, Plus, SearchX, Trash2, X } from "lucide-react";

import { Alert, AlertAction, AlertDescription, Button, ConfirmDialog, Empty } from "@compozy/ui";

import { useCreateProviderFocusRestore } from "@/hooks/routes/use-create-provider-focus-restore";
import {
  useSettingsProvidersPage,
  type ProviderLastAction,
} from "@/systems/settings/hooks/use-settings-providers-page";
import {
  ProviderCard,
  ProviderDetailDialog,
  ProvidersToolbar,
  SettingsPageFrame,
  SettingsPageState,
  useSettingsTopbar,
  type SettingsProviderEntry,
} from "@/systems/settings";

export function ProvidersSettingsPage() {
  const page = useSettingsProvidersPage();
  const createProviderButtonRef = useCreateProviderFocusRestore(page.inspector.mode);
  useSettingsTopbar({
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
    return <SettingsPageState slug="providers" state="loading" />;
  }

  if (page.error || !page.envelope) {
    return (
      <SettingsPageState error={page.error} onRetry={page.retry} slug="providers" state="error" />
    );
  }

  return (
    <SettingsPageFrame
      meta={providerCountMeta(page.counts)}
      restart={page.restart}
      slug="providers"
      width="wide"
    >
      {page.lastAction ? (
        <LastActionAlert action={page.lastAction} onDismiss={page.dismissLastAction} />
      ) : null}

      <ProvidersCatalog page={page} />

      <ProvidersInspector page={page} />

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

type ProvidersPage = ReturnType<typeof useSettingsProvidersPage>;

function providerCountMeta(counts: ProvidersPage["counts"]) {
  return [
    {
      key: "ready",
      content: <span data-testid="settings-page-providers-ready">{counts.installed} ready</span>,
    },
    {
      key: "setup",
      content: (
        <span data-testid="settings-page-providers-needs-setup">
          {counts.needsSetup} needs setup
        </span>
      ),
    },
    {
      key: "missing",
      content: (
        <span data-testid="settings-page-providers-missing">
          {counts.binaryMissing} not installed
        </span>
      ),
    },
  ];
}

function ProvidersCatalog({ page }: { page: ProvidersPage }) {
  if (page.providers.length === 0) {
    return (
      <Empty
        data-testid="settings-page-providers-empty"
        description="Add one to start sessions."
        icon={Database}
        title="No providers yet"
      />
    );
  }
  return (
    <>
      <ProvidersToolbar
        nameQuery={page.filters.nameQuery}
        onNameQueryChange={page.setNameQuery}
        onStatusChange={page.setStatusFilter}
        statusFilter={page.filters.statusFilter}
      />
      {page.filteredProviders.length === 0 ? (
        <Empty
          action={
            <Button
              onClick={() => {
                page.setNameQuery("");
                page.setStatusFilter(null);
              }}
              size="sm"
              type="button"
              variant="secondary"
            >
              Clear filters
            </Button>
          }
          data-testid="settings-page-providers-empty-filtered"
          description="Try a different search or status."
          framed
          icon={SearchX}
          title="No providers match"
        />
      ) : (
        <section
          className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
          data-testid="settings-page-providers-list"
        >
          {page.filteredProviders.map(provider => (
            <ProviderCard key={provider.name} onOpen={page.openInspect} provider={provider} />
          ))}
        </section>
      )}
    </>
  );
}

function ProvidersInspector({ page }: { page: ProvidersPage }) {
  const { inspector } = page;
  const inspectorEntry =
    inspector.mode === "inspect" || inspector.mode === "edit" ? inspector.entry : null;
  const inspectorDraft =
    inspector.mode === "edit" || inspector.mode === "create" ? inspector.draft : null;

  return (
    <ProviderDetailDialog
      open={inspector.mode !== "closed"}
      mode={inspector.mode === "closed" ? "inspect" : inspector.mode}
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
  const name = target ? providerDisplayName(target) : "";

  return (
    <ConfirmDialog
      open={open}
      title={
        !target ? "Delete provider" : fallback ? `Reset "${name}"?` : `Delete provider "${name}"?`
      }
      description={
        target
          ? fallback
            ? "Your changes to this provider are removed."
            : "This removes the provider from your settings."
          : null
      }
      note={
        fallback ? (
          <div className="flex flex-col gap-1" data-testid="settings-providers-delete-builtin">
            <span className="font-medium">It stays available</span>
            <span>The provider goes back to the setup CompozyOS ships with.</span>
          </div>
        ) : null
      }
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel={fallback ? "Reset to default" : "Delete provider"}
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
      ? `Reset "${action.name}" to its default setup · ${restartBadge}.`
      : `Deleted provider "${action.name}" · ${restartBadge}.`;

  return (
    <Alert
      variant={isSaved ? "success" : "info"}
      role="status"
      data-testid="settings-page-providers-action-result"
      data-kind={action.kind}
    >
      <Check aria-hidden="true" className="size-3" />
      <AlertDescription className="text-form-hint">{message}</AlertDescription>
      <AlertAction>
        <Button
          aria-label="Dismiss"
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

function providerDisplayName(entry: SettingsProviderEntry): string {
  return entry.settings.display_name?.trim() || entry.name;
}
