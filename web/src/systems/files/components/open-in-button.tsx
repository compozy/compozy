import type { ReactNode } from "react";
import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { ChevronDown, Code2, ExternalLink, FolderOpen } from "@compozy/ui/icons";
import { CursorLogo } from "@compozy/ui/logos";

import { desktopShellBridge, type OpenTarget } from "@/systems/os";

import type { FilesRootRef } from "../adapters/files-api";
import { useOpenIn } from "../hooks/use-open-in";
import {
  canOpenInTargets,
  openTargetActionLabel,
  type OpenInRequest,
} from "../lib/open-in-targets";
import { FileMenuItems, OpenInMenuItems } from "./open-in-menu-items";

/** Brand marks that exist in `@compozy/ui/logos`; other editors use the generic code icon. */
function OpenTargetIcon({ target }: { target: OpenTarget }) {
  if (target.kind === "reveal") return <FolderOpen aria-hidden />;
  if (target.id === "cursor") return <CursorLogo aria-hidden className="size-4" />;
  return <Code2 aria-hidden />;
}

/**
 * The viewer's Open in split button (`_uiux.md` S5): desktop app only, absent elsewhere
 * (US-006.AC-6). The primary half opens with the last-used target.
 */
export function OpenInButton({
  root,
  path,
  line,
  children,
}: {
  root?: FilesRootRef;
  path?: string;
  line?: number;
  children?: ReactNode;
}) {
  if (!desktopShellBridge()?.files?.listOpenTargets) return null;
  if (!root || !path) return <>{children}</>;
  if (!canOpenInTargets()) return null;
  return <OpenTargetButton root={root} request={{ path, line }} copyPath={path} />;
}

function OpenTargetButton({
  root,
  request,
  copyPath,
}: {
  root: FilesRootRef;
  request: OpenInRequest;
  copyPath: string;
}) {
  const openIn = useOpenIn(root);
  const primary = openIn.primary;
  return (
    <div className="flex items-center" data-testid="open-in-split-button">
      {primary ? (
        <Button
          size="sm"
          variant="outline"
          disabled={openIn.busy}
          onClick={() => void openIn.open(primary, request)}
        >
          <OpenTargetIcon target={primary} />
          {openTargetActionLabel(primary)}
        </Button>
      ) : null}
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              size="sm"
              variant="outline"
              aria-label="Choose application"
              disabled={openIn.busy}
            />
          }
        >
          <ChevronDown />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-auto">
          <OpenInMenuItems host="dropdown" openIn={openIn} request={request} copyPath={copyPath} />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

/**
 * The tree root header's Open in menu: the root directory, without a line (US-006.AC-4). The
 * root has no root-relative path, so its menu has no Copy path.
 */
export function OpenRootInMenu({ root }: { root: FilesRootRef }) {
  if (!canOpenInTargets()) return null;
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  variant="quiet"
                  size="icon-xs"
                  aria-label="Open folder in…"
                  data-testid="files-root-open-in"
                />
              }
            />
          }
        >
          <ExternalLink aria-hidden className="size-3.5" />
        </TooltipTrigger>
        <TooltipContent>Open folder in…</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" className="w-auto">
        <FileMenuItems host="dropdown" root={root} request={{ path: "", directory: true }} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
