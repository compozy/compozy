import { MenuTreeItems, type MenuTreeHost } from "@compozy/ui";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { useCurrentComputerId } from "@/lib/computer-scope-context";
import type { FilesRootRef } from "../adapters/files-api";
import { useOpenIn } from "../hooks/use-open-in";
import { fileMenuNodes, type FileMenuInput } from "../lib/file-menu";
import { canOpenInTargets } from "../lib/open-in-targets";

type FileMenuProps = Omit<FileMenuInput, "openIn"> & {
  host: MenuTreeHost;
  root: FilesRootRef;
  computerId?: ComputerId;
};

/** The file menu where the host already holds the Open in targets (the viewer's split button). */
export function OpenInMenuItems({ host, ...input }: FileMenuInput & { host: MenuTreeHost }) {
  return <MenuTreeItems host={host} nodes={fileMenuNodes(input)} />;
}

/**
 * The file menu for one path under `root`. The Open in targets load only in the desktop app and
 * only for local roots (US-012.AC-2, UT-090); remote roots and browser tabs keep Copy path.
 */
export function FileMenuItems({ root, computerId, ...props }: FileMenuProps) {
  const effectiveComputerId = useCurrentComputerId(computerId ?? root.computerId);
  if (isRemoteComputerId(effectiveComputerId) || !canOpenInTargets()) {
    return <OpenInMenuItems {...props} openIn={null} />;
  }
  return <DesktopFileMenuItems root={root} {...props} />;
}

function DesktopFileMenuItems({ root, ...props }: FileMenuProps) {
  const openIn = useOpenIn(root);
  return <OpenInMenuItems {...props} openIn={openIn} />;
}
