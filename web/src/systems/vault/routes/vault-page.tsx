import { AlertCircle, KeyRound, Lock, Plus, RefreshCw, Trash2 } from "lucide-react";

import {
  Button,
  ConfirmDialog,
  Empty,
  ListingPage,
  ListingToolbar,
  MonoId,
  Skeleton,
  SkeletonRows,
  Spinner,
  useTopbarSlot,
} from "@compozy/ui";

import { useVaultPage } from "../hooks/use-vault-page";
import { vaultSecretTitle } from "../lib/vault-secret-title";
import type { VaultRouteSearch } from "../lib/vault-route-search";
import { VaultEditor } from "../components/vault-editor";
import { VaultListFilters } from "../components/vault-list-filters";
import { VaultSecretSheet } from "../components/vault-secret-sheet";
import { VaultSecretsList } from "../components/vault-secrets-list";
import type { VaultSecret } from "../types";

type VaultPageModel = ReturnType<typeof useVaultPage>;

export function VaultPage({ search = {} }: { search?: VaultRouteSearch }) {
  const page = useVaultPage(search);

  useTopbarSlot({
    glyph: <KeyRound />,
    count: page.isLoading ? undefined : page.counts.total,
    crumb: "Vault",
    actions: <VaultTopbarActions page={page} />,
    toolbar: page.isLoading ? undefined : <VaultListingToolbar page={page} />,
  });

  if (page.isLoading) {
    return <VaultPageLoading />;
  }

  return (
    <ListingPage data-testid="vault-shell">
      <p
        className="mb-3 flex items-center gap-2 text-eyebrow text-subtle"
        data-testid="vault-page-sec-note"
      >
        <Lock aria-hidden="true" className="size-3 shrink-0 text-faint" />
        Values are encrypted. You can't view a secret after you save it.
      </p>

      <VaultPageContent page={page} />

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

function VaultRefreshIcon({ isRefetching }: { isRefetching: boolean }) {
  return isRefetching ? <Spinner className="size-3.5" /> : <RefreshCw />;
}

function VaultTopbarActions({ page }: { page: VaultPageModel }) {
  return (
    <div className="flex items-center gap-2" data-testid="vault-topbar-actions">
      <Button
        data-testid="vault-page-refresh"
        disabled={page.isRefetching}
        onClick={() => void page.refetch()}
        size="sm"
        type="button"
        variant="ghost"
      >
        <VaultRefreshIcon isRefetching={page.isRefetching} />
        Refresh
      </Button>
      <Button
        data-testid="vault-page-create"
        onClick={page.openCreate}
        size="sm"
        type="button"
        variant="secondary"
      >
        <Plus />
        New secret
      </Button>
    </div>
  );
}

function VaultListingToolbar({ page }: { page: VaultPageModel }) {
  return (
    <ListingToolbar>
      <ListingToolbar.Leading>
        <ListingToolbar.Search
          aria-label="Search secrets"
          data-testid="vault-page-prefix"
          onChange={page.setPrefix}
          placeholder="Search secrets"
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
  );
}

function VaultPageLoading() {
  return (
    <ListingPage data-testid="vault-page-loading">
      <div aria-label="Loading secrets" role="status">
        <SkeletonRows
          className="overflow-hidden rounded-lg bg-card shadow-card"
          count={4}
          rowClassName="flex-row items-center gap-3 border-b border-line-soft px-4 py-3 last:border-b-0"
        >
          <Skeleton className="size-8 shrink-0 rounded-md" />
          <div className="flex min-w-0 flex-1 flex-col gap-1.5">
            <Skeleton className="h-3 w-2/5 rounded-xs" />
            <Skeleton className="h-2.5 w-1/4 rounded-xs" />
          </div>
        </SkeletonRows>
      </div>
    </ListingPage>
  );
}

/** Deep-link failure wins, then a blocking list error, then the list itself. */
function VaultPageContent({ page }: { page: VaultPageModel }) {
  if (page.selectionError) {
    return (
      <Empty
        data-testid="vault-page-selection-error"
        description="The requested secret was deleted or is unavailable."
        icon={AlertCircle}
        title={page.selectionError}
      />
    );
  }
  if (page.queryError && page.secrets.length === 0) {
    return (
      <Empty
        action={
          <Button
            data-testid="vault-page-error-retry"
            disabled={page.isRefetching}
            onClick={() => void page.refetch()}
            size="sm"
            type="button"
            variant="secondary"
          >
            <VaultRefreshIcon isRefetching={page.isRefetching} />
            Retry
          </Button>
        }
        data-testid="vault-page-error"
        description={page.queryError}
        icon={AlertCircle}
        title="Couldn't load your secrets"
      />
    );
  }
  return (
    <VaultSecretsList
      data-testid="vault-page-list"
      emptyAction={
        <Button
          data-testid="vault-page-empty-create"
          onClick={page.openCreate}
          size="sm"
          type="button"
        >
          <Plus />
          New secret
        </Button>
      }
      emptyDescription="Add an API key or token so agents and extensions can use it."
      emptyTitle="No secrets yet"
      error={page.queryError ? new Error(page.queryError) : null}
      isLoading={page.isRefetching && page.secrets.length === 0}
      onDelete={page.openDelete}
      onSelect={page.openInspect}
      secrets={page.secrets}
      selectedRef={page.selectedSecret?.ref ?? null}
      view={page.view}
    />
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
  const title = target ? vaultSecretTitle(target.ref) : "";
  const confirmTypingValue = target && !sessionScope ? title : undefined;
  return (
    <ConfirmDialog
      open={target !== null}
      title={title ? `Delete ${title}?` : "Delete secret?"}
      description={
        target ? (
          <span className="flex flex-col gap-1">
            <span>Anything that uses {title} stops working.</span>
            <MonoId preserveCase value={target.ref} />
          </span>
        ) : null
      }
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel="Delete secret"
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
