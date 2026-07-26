import { AlertCircle, Check, KeyRound, Lock, Plus, RefreshCw, Trash2, X } from "lucide-react";

import {
  Alert,
  AlertAction,
  AlertDescription,
  Button,
  ConfirmDialog,
  Empty,
  ListingPage,
  ListingToolbar,
  Skeleton,
  useTopbarSlot,
} from "@agh/ui";

import { useVaultPage, type VaultLastAction, type VaultRouteSearch } from "../hooks/use-vault-page";
import { VaultEditor } from "../components/vault-editor";
import { VaultListFilters } from "../components/vault-list-filters";
import { VaultSecretSheet } from "../components/vault-secret-sheet";
import { VaultSecretsList } from "../components/vault-secrets-list";
import type { VaultSecret } from "../types";

export function VaultPage({ search = {} }: { search?: VaultRouteSearch }) {
  const page = useVaultPage(search);

  useTopbarSlot({
    glyph: <KeyRound />,
    count: page.isLoading ? undefined : page.counts.total,
    crumb: "Vault",
    actions: (
      <div className="flex items-center gap-2" data-testid="vault-topbar-actions">
        <Button
          data-testid="vault-page-refresh"
          disabled={page.isRefetching}
          onClick={() => void page.refetch()}
          size="sm"
          type="button"
          variant="ghost"
        >
          <RefreshCw className={page.isRefetching ? "size-3 animate-spin" : "size-3"} />
          Refresh
        </Button>
        <Button data-testid="vault-page-create" onClick={page.openCreate} size="sm" type="button">
          <Plus className="size-3" />
          New secret
        </Button>
      </div>
    ),
    toolbar: page.isLoading ? undefined : (
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search
            aria-label="Filter by vault ref prefix"
            data-testid="vault-page-prefix"
            onChange={page.setPrefix}
            placeholder="Filter by ref prefix"
            value={page.prefix}
          />
          <ListingToolbar.Filters>
            <VaultListFilters namespace={page.namespace} onNamespaceChange={page.setNamespace} />
          </ListingToolbar.Filters>
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={page.setView} value={page.view} />
        </ListingToolbar.Trailing>
      </ListingToolbar>
    ),
  });

  if (page.isLoading) {
    return (
      <ListingPage data-testid="vault-page-loading">
        <div aria-label="Loading vault metadata" className="flex flex-col gap-3" role="status">
          <Skeleton className="h-4 w-3/5 rounded-xs" />
          <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
            {[0, 1, 2, 3].map(row => (
              <div
                className="flex items-center gap-3 border-b border-line-soft px-4 py-3 last:border-b-0"
                key={row}
              >
                <Skeleton className="size-8 shrink-0 rounded-md" />
                <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                  <Skeleton className="h-3 w-2/5 rounded-xs" />
                  <Skeleton className="h-2.5 w-3/5 rounded-xs" />
                </div>
                <Skeleton className="h-5 w-20 rounded-pill" />
              </div>
            ))}
          </div>
        </div>
      </ListingPage>
    );
  }

  return (
    <ListingPage
      banner={
        page.lastAction ? (
          <div className="px-9 pt-4">
            <LastActionAlert action={page.lastAction} onDismiss={page.dismissLastAction} />
          </div>
        ) : null
      }
      data-testid="vault-shell"
    >
      <p
        className="mb-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-subtle"
        data-testid="vault-page-sec-note"
      >
        <Lock aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
        <span>
          {page.counts.total} redacted metadata {page.counts.total === 1 ? "entry" : "entries"} —
          values are write-only and never leave the daemon.
        </span>
        <span aria-hidden="true" className="text-faint">
          ·
        </span>
        <span data-testid="vault-page-count">{page.counts.total}</span>
        <span aria-hidden="true" className="text-faint">
          ·
        </span>
        <span data-testid="vault-page-sessions">{page.counts.sessions} session-scoped</span>
        <span aria-hidden="true" className="text-faint">
          ·
        </span>
        <span data-testid="vault-page-providers">{page.counts.providers} provider-scoped</span>
      </p>

      {page.queryError && page.secrets.length === 0 ? (
        <Empty
          action={
            <Button
              data-testid="vault-page-error-retry"
              disabled={page.isRefetching}
              onClick={() => void page.refetch()}
              size="sm"
              type="button"
              variant="ghost"
            >
              <RefreshCw className={page.isRefetching ? "size-3 animate-spin" : "size-3"} />
              Retry
            </Button>
          }
          data-testid="vault-page-error"
          description={page.queryError}
          icon={AlertCircle}
          title="Unable to load vault metadata"
        />
      ) : (
        <VaultSecretsList
          data-testid="vault-page-list"
          emptyDescription="Vault metadata appears here after a write-only secret is stored."
          emptyTitle="No vault secrets"
          error={page.queryError ? new Error(page.queryError) : null}
          isLoading={page.isRefetching && page.secrets.length === 0}
          onDelete={page.openDelete}
          onSelect={page.openInspect}
          secrets={page.secrets}
          selectedRef={page.selectedSecret?.ref ?? null}
          view={page.view}
        />
      )}

      <VaultSecretSheet
        deleteIsDisabled={page.replaceIsPending}
        onOpenChange={open => {
          if (!open) page.closeInspect();
        }}
        onReplace={page.replaceSecret}
        onReplaceValueChange={page.setReplaceValue}
        onRequestDelete={page.openDelete}
        open={page.selectedSecret !== null}
        replaceError={page.replaceError}
        replaceIsPending={page.replaceIsPending}
        replaceIsValid={page.replaceIsValid}
        replaceValue={page.replaceValue}
        secret={page.selectedSecret}
      />

      <VaultEditor
        canSave={page.editorIsValid}
        editor={page.editor}
        error={page.editorError}
        isSaving={page.editorIsSaving}
        onChange={page.updateDraft}
        onClose={page.closeEditor}
        onSave={page.saveEditor}
        refExists={page.editorRefExists}
      />

      <VaultDeleteDialog
        error={page.deleteError}
        isDeleting={page.deleteIsPending}
        onClose={page.closeDelete}
        onConfirm={page.confirmDelete}
        target={page.deleteTarget.mode === "open" ? page.deleteTarget.secret : null}
      />
    </ListingPage>
  );
}

interface VaultDeleteDialogProps {
  target: VaultSecret | null;
  error: string | null;
  isDeleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

function isSessionScopedVaultRef(ref: string): boolean {
  return ref.startsWith("vault:sessions/");
}

function VaultDeleteDialog({
  target,
  error,
  isDeleting,
  onClose,
  onConfirm,
}: VaultDeleteDialogProps) {
  const sessionScope = target ? isSessionScopedVaultRef(target.ref) : false;
  const confirmTypingValue = target && !sessionScope ? target.ref : undefined;
  return (
    <ConfirmDialog
      open={target !== null}
      title={sessionScope ? "Delete session vault secret?" : "Delete vault secret?"}
      description={
        target ? (
          <span>
            Delete metadata and encrypted value for{" "}
            <code className="font-mono text-fg">{target.ref}</code>.
            {sessionScope
              ? " This is a session-scoped secret; it is removed immediately."
              : " Cross-scope vault entries require typed confirmation."}
          </span>
        ) : null
      }
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel={sessionScope ? "Confirm" : "Delete secret"}
      confirmIcon={Trash2}
      confirmTyping={confirmTypingValue}
      contentProps={{
        "data-testid": "settings-vault-delete",
        "data-scope": sessionScope ? "session" : "cross",
      }}
      descriptionProps={{ "data-testid": "settings-vault-delete-description" }}
      errorProps={{ "data-testid": "settings-vault-delete-error" }}
      cancelButtonProps={{
        "data-testid": "settings-vault-delete-cancel",
        disabled: isDeleting,
      }}
      confirmButtonProps={{
        "data-testid": "settings-vault-delete-confirm",
      }}
      confirmInputProps={{ "data-testid": "settings-vault-delete-confirm-typing" }}
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
  action: VaultLastAction;
  onDismiss: () => void;
}) {
  const saved = action.kind === "saved";
  return (
    <Alert variant={saved ? "success" : "warning"} data-testid="vault-page-action-result">
      {saved ? <Check className="size-4" /> : <KeyRound className="size-4" />}
      <AlertDescription>
        {saved ? "Stored vault metadata for " : "Deleted vault secret "}
        <code className="font-mono">{action.ref}</code>.
      </AlertDescription>
      <AlertAction>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          aria-label="Dismiss vault action result"
          onClick={onDismiss}
          data-testid="vault-page-action-result-dismiss"
        >
          <X className="size-3" />
        </Button>
      </AlertAction>
    </Alert>
  );
}
