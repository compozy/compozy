import { DropdownMenuItem, DropdownMenuSeparator, type FileTreeRowMenuItem } from "@compozy/ui";
import { Copy } from "@compozy/ui/icons";

import { desktopShellBridge } from "@/systems/os";

import type { FilesRootRef } from "../adapters/files-api";
import { useOpenIn } from "../hooks/use-open-in";
import { copyFilePath } from "../lib/copy-file-path";
import { OpenInMenuItems } from "./open-in-menu-items";

function OpenInRowItems({ root, item }: { root: FilesRootRef; item: FileTreeRowMenuItem }) {
  const openIn = useOpenIn(root);
  return (
    <>
      <DropdownMenuSeparator />
      <OpenInMenuItems
        openIn={openIn}
        request={{ path: item.path, directory: item.kind === "directory" }}
      />
    </>
  );
}

/**
 * A tree row's context menu (`_uiux.md` S3): Copy path everywhere, plus the Open in targets in
 * the desktop app. Directories open without a line (US-006.AC-4).
 */
export function FilesTreeRowMenu({
  root,
  item,
}: {
  root: FilesRootRef;
  item: FileTreeRowMenuItem;
}) {
  const files = desktopShellBridge()?.files;
  const desktop = Boolean(files?.listOpenTargets && files.openWith);
  return (
    <>
      <DropdownMenuItem onClick={() => copyFilePath(item.path.replace(/\/$/, ""))}>
        <Copy aria-hidden />
        Copy path
      </DropdownMenuItem>
      {desktop ? <OpenInRowItems root={root} item={item} /> : null}
    </>
  );
}
