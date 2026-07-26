import { AlertCircle, Boxes, Info, Plus, RefreshCw } from "lucide-react";

import { Button, Empty, ListingPage, ListingToolbar, SkeletonRows, useTopbarSlot } from "@agh/ui";

import { useSandboxPage, type SandboxRouteSearch } from "../hooks/use-sandbox-page";
import { SandboxListFilters, SandboxProfilesList, SandboxProfileSheet } from "../components";
import { SandboxEditor } from "../components/sandbox-editor";
import { SettingsRestartNotice } from "@/systems/settings";
import { SandboxDeleteDialog, SandboxLastActionAlert } from "../components/sandbox-dialogs";

export function SandboxPage({ search = {} }: { search?: SandboxRouteSearch }) {
  const page = useSandboxPage(search);

  useTopbarSlot({
    glyph: <Boxes />,
    count: page.isLoading ? undefined : page.counts.total,
    crumb: "Sandbox",
    actions: (
      <div className="flex items-center gap-2" data-testid="sandbox-topbar-actions">
        <Button
          data-testid="sandbox-page-refresh"
          disabled={page.isRefetching}
          onClick={() => void page.refetch()}
          size="sm"
          type="button"
          variant="ghost"
        >
          <RefreshCw className={page.isRefetching ? "size-3 animate-spin" : "size-3"} />
          Refresh
        </Button>
        <Button data-testid="sandbox-page-create" onClick={page.openCreate} size="sm" type="button">
          <Plus className="size-3" />
          New sandbox profile
        </Button>
      </div>
    ),
    toolbar: page.isLoading ? undefined : (
      <ListingToolbar>
        <ListingToolbar.Leading>
          <ListingToolbar.Search
            aria-label="Search profiles"
            data-testid="sandbox-page-search"
            onChange={page.setQuery}
            placeholder="Search profiles"
            value={page.query}
          />
          <ListingToolbar.Filters>
            <SandboxListFilters
              backend={page.backend}
              onBackendChange={page.setBackend}
              onPersistenceChange={page.setPersistence}
              persistence={page.persistence}
            />
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
      <SkeletonRows
        aria-label="Loading sandbox profiles"
        className="min-h-0 flex-1 p-5"
        count={6}
        data-testid="sandbox-page-loading"
        role="status"
        rowClassName="border-b border-line-soft py-3"
      />
    );
  }

  const banner = (
    <>
      {page.restart.isVisible ? (
        <div className="px-9 pt-4">
          <SettingsRestartNotice restart={page.restart} slug="sandbox" />
        </div>
      ) : null}
      {page.lastAction ? (
        <div className="px-9 pt-4">
          <SandboxLastActionAlert action={page.lastAction} onDismiss={page.dismissLastAction} />
        </div>
      ) : null}
    </>
  );

  return (
    <ListingPage banner={banner} data-testid="sandbox-shell">
      <p
        className="mb-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-subtle"
        data-testid="sandbox-page-sec-note"
      >
        <Info aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
        <span>
          Profiles are global. When{" "}
          <span className="font-mono text-inline-code">defaults.sandbox</span> is unset, sessions
          fall back to a synthetic local profile.
        </span>
        <span aria-hidden="true" className="text-faint">
          ·
        </span>
        <span data-testid="sandbox-page-total">
          {page.counts.total} {page.counts.total === 1 ? "profile" : "profiles"}
        </span>
        <span aria-hidden="true" className="text-faint">
          ·
        </span>
        <span data-testid="sandbox-page-workspaces">
          {page.counts.totalWorkspaces} workspace references
        </span>
      </p>

      {page.queryError && page.sandboxes.length === 0 ? (
        <Empty
          action={
            <Button
              data-testid="sandbox-page-error-retry"
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
          data-testid="sandbox-page-error"
          description={
            page.queryError ?? "The daemon stopped responding before it returned the profile list."
          }
          icon={AlertCircle}
          title="Failed to load sandboxes"
        />
      ) : (
        <SandboxProfilesList
          data-testid="sandbox-page-list"
          error={page.queryError ? new Error(page.queryError) : null}
          hasActiveFilters={page.hasActiveFilters}
          onClearFilters={page.clearFilters}
          onCreate={page.openCreate}
          onDelete={page.openDelete}
          onEdit={page.openEdit}
          onSelect={page.openInspect}
          profiles={page.filtered}
          selectedName={page.selectedEntry?.name ?? null}
          view={page.view}
        />
      )}

      <SandboxProfileSheet
        entry={page.selectedEntry}
        onOpenChange={open => {
          if (!open) page.closeInspect();
        }}
        onRequestDelete={page.openDelete}
        onRequestEdit={page.openEdit}
        open={page.selectedEntry !== null}
      />

      <SandboxEditor
        editor={page.editor}
        error={page.editorError}
        existingNames={page.sandboxes.map(entry => entry.name)}
        isSaving={page.editorIsSaving}
        isValid={page.editorIsValid}
        onChange={page.updateDraft}
        onClose={page.closeEditor}
        onSave={page.saveEditor}
        warnings={page.editorWarnings}
      />

      <SandboxDeleteDialog
        error={page.deleteError}
        isDeleting={page.deleteIsPending}
        onClose={page.closeDelete}
        onConfirm={page.confirmDelete}
        target={page.deleteTarget.mode === "open" ? page.deleteTarget.entry : null}
      />
    </ListingPage>
  );
}
