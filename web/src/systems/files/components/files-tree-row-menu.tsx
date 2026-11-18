import type { FileTreeRowMenuItem } from "@compozy/ui";

import type { FilesRootRef } from "../adapters/files-api";
import { FileMenuItems } from "./open-in-menu-items";

/**
 * A tree row's context menu (`_uiux.md` S3): the shared file menu, with the Open in targets in
 * the desktop app and Copy path everywhere. Directories open without a line (US-006.AC-4).
 */
export function FilesTreeRowMenu({
  root,
  item,
}: {
  root: FilesRootRef;
  item: FileTreeRowMenuItem;
}) {
  return (
    <FileMenuItems
      host="dropdown"
      root={root}
      request={{ path: item.path, directory: item.kind === "directory" }}
      copyPath={item.path.replace(/\/$/, "")}
    />
  );
}
