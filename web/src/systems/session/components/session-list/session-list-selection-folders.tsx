import { FolderInput } from "@compozy/ui/icons";

import { Pill } from "@compozy/ui";

import type { SessionListBulkFolders } from "../../hooks/use-session-list-bulk-folders";
import { sessionCountLabel } from "../../lib/session-folders";
import { SessionFolderDialogs } from "./session-folder-dialogs";

/**
 * What a selection's folder moves render outside the bar: the count the
 * browser shows under the pointer while a selection drags, and the New folder
 * dialog that moves the selection into the folder it creates.
 */
export function SessionListSelectionFolders({
  folders,
  active,
}: {
  folders: SessionListBulkFolders;
  active: boolean;
}) {
  const { dialog, setPreview, previewCount } = folders;
  return (
    <>
      {active ? (
        // The browser snapshots this element when a selected row starts dragging,
        // so it stays rendered, off-screen, while a selection is open.
        <div aria-hidden="true" className="pointer-events-none fixed top-0 -left-full">
          <div ref={setPreview} className="inline-flex p-1">
            <Pill tone="brand" solid size="md">
              <FolderInput aria-hidden="true" className="size-3" />
              {sessionCountLabel(previewCount)}
            </Pill>
          </div>
        </div>
      ) : null}
      {dialog ? (
        <SessionFolderDialogs
          state={dialog.open ? { kind: "create", moveCount: dialog.count } : null}
          onClose={dialog.onClose}
          onCreate={dialog.onCreate}
          onRename={dialog.layout.renameFolder}
          onDelete={dialog.layout.deleteFolder}
        />
      ) : null}
    </>
  );
}
