import { Plus } from "lucide-react";
import { useState } from "react";

import { ConfirmDialog } from "@agh/ui";

import type { WindowManagerLayoutProfilesModel } from "../../hooks/use-window-manager-layout-profiles";
import { isSelectedLayoutProfile } from "../../lib/window-manager-layout-profile-key";
import type { WindowManagerLayoutDocument } from "../../lib/window-manager-layout-types";
import { LayoutProfileCard } from "./layout-profile-card";
import { LayoutProfileEditor } from "./layout-profile-editor";

interface LayoutProfileGridProps {
  editor: WindowManagerLayoutProfilesModel;
  document: WindowManagerLayoutDocument;
}

/**
 * Saved layouts as cards, plus the one form that edits them. Both destructive
 * paths — replacing unapplied edits, and deleting a record other people can see —
 * ask first; the hook decides, this only renders the question.
 */
export function LayoutProfileGrid({ editor, document }: LayoutProfileGridProps) {
  const [editing, setEditing] = useState(false);

  return (
    <div className="flex flex-col gap-2.5" data-testid="layout-profile-grid">
      <div className="grid gap-2.5 [grid-template-columns:repeat(auto-fill,minmax(13rem,1fr))]">
        {editor.profiles.map(record => (
          <LayoutProfileCard
            key={`${record.scope.kind}:${record.scope.id}:${record.id}`}
            record={record}
            selected={isSelectedLayoutProfile(editor.selectedKey, record)}
            onDelete={() => {
              editor.selectProfile(record);
              editor.requestDelete();
            }}
            onLoad={() => editor.requestLoad(record)}
            onSelect={() => {
              editor.selectProfile(record);
              setEditing(true);
            }}
          />
        ))}
        <button
          className="flex min-h-50 flex-col items-center justify-center gap-2 rounded-lg border border-dashed border-line text-small-body text-muted transition-colors duration-base ease-out hover:border-line-strong hover:text-fg-strong focus-visible:outline-none focus-visible:shadow-focus-ring"
          data-testid="layout-profile-new"
          type="button"
          onClick={() => {
            editor.startNew();
            setEditing(true);
          }}
        >
          <Plus aria-hidden="true" className="size-4.5" />
          Save the current layout
        </button>
      </div>

      {editing ? (
        <LayoutProfileEditor
          document={document}
          editor={editor}
          onClose={() => setEditing(false)}
        />
      ) : null}

      <ConfirmDialog
        cancelLabel="Keep editing"
        confirmLabel="Replace edits"
        description={
          <>
            Loading <b>{editor.pendingLoad?.spec.displayName}</b> replaces the layout edits you have
            not applied yet.
          </>
        }
        open={editor.pendingLoad !== null}
        title="Replace unapplied layout edits?"
        tone="warning"
        onConfirm={editor.confirmLoad}
        onOpenChange={open => {
          if (!open) editor.cancelLoad();
        }}
      />

      <ConfirmDialog
        cancelLabel="Cancel"
        confirmLabel="Delete layout"
        description={
          <>
            <b>{editor.pendingDelete?.spec.displayName}</b> is removed for everyone who can see it.
            The layout in the editor is untouched.
          </>
        }
        isPending={editor.remove.isPending}
        open={editor.pendingDelete !== null}
        title="Delete this saved layout?"
        onConfirm={editor.confirmDelete}
        onOpenChange={open => {
          if (!open) editor.cancelDelete();
        }}
      />
    </div>
  );
}
