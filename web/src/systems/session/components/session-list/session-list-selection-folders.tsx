import { Check, Folder, FolderInput, FolderOutput, FolderPlus } from "lucide-react";

import { Pill } from "@compozy/ui";

import type {
  SessionBulkFolderMenu,
  SessionListBulkFolders,
} from "../../hooks/use-session-list-bulk-folders";
import { sessionCountLabel } from "../../lib/session-folders";
import { SessionFolderDialogs } from "./session-folder-dialogs";
import type { SessionSelectionMenuKit } from "./session-list-selection-menu";

/**
 * Move to folder in a selection menu: the folders, No folder when a
 * selected session is filed, and New folder… to create one around the
 * selection. A selection that cannot move says why in place of the targets.
 */
export function SessionSelectionFolderMenu({
  kit,
  menu,
  disabled,
  testId,
}: {
  kit: SessionSelectionMenuKit;
  menu: SessionBulkFolderMenu;
  disabled: boolean;
  testId: string;
}) {
  const { Item, Separator, Sub, SubTrigger, SubContent } = kit;
  return (
    <Sub>
      <SubTrigger disabled={disabled} data-testid={testId}>
        <FolderInput aria-hidden="true" className="size-3" />
        Move to folder
      </SubTrigger>
      <SubContent className="max-w-64 min-w-44">
        {menu.blockedReason !== null ? (
          // Focusable but inert, unlike a disabled item: keyboard and screen
          // reader users land on the reason instead of an empty submenu.
          <Item
            closeOnClick={false}
            className="text-muted focus:text-muted"
            data-testid={`${testId}-blocked`}
          >
            {menu.blockedReason}
          </Item>
        ) : (
          <>
            {menu.folders.map(folder => {
              const current = folder.id === menu.sharedFolderId;
              return (
                <Item
                  key={folder.id}
                  disabled={current}
                  onClick={() => menu.onMove(folder.id)}
                  data-testid={`${testId}-${folder.id}`}
                >
                  {current ? (
                    <Check aria-hidden="true" className="size-3" />
                  ) : (
                    <Folder aria-hidden="true" className="size-3" />
                  )}
                  <span className="min-w-0 truncate">{folder.name}</span>
                </Item>
              );
            })}
            {menu.canUnfile ? (
              <Item onClick={() => menu.onMove(null)} data-testid={`${testId}-root`}>
                <FolderOutput aria-hidden="true" className="size-3" />
                No folder
              </Item>
            ) : null}
            {menu.folders.length > 0 || menu.canUnfile ? <Separator /> : null}
            <Item onClick={menu.onNewFolder} data-testid={`${testId}-new`}>
              <FolderPlus aria-hidden="true" className="size-3" />
              New folder…
            </Item>
          </>
        )}
      </SubContent>
    </Sub>
  );
}

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
            <Pill tone="accent" solid size="md">
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
