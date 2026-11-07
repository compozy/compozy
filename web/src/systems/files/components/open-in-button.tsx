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
import { ChevronDown, ExternalLink } from "@compozy/ui/icons";

import { desktopShellBridge } from "@/systems/os";

import type { FilesRootRef } from "../adapters/files-api";
import { useOpenIn, type OpenInRequest } from "../hooks/use-open-in";
import { openTargetActionLabel } from "../lib/open-in-targets";
import { OpenInMenuItems, OpenTargetIcon } from "./open-in-menu-items";

function hasOpenInBridge(): boolean {
  const files = desktopShellBridge()?.files;
  return Boolean(files?.listOpenTargets && files.openWith);
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
  if (!hasOpenInBridge()) return null;
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
          <OpenInMenuItems openIn={openIn} request={request} copyPath={copyPath} />
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

/** The tree root header's Open in menu: the root directory, without a line (US-006.AC-4). */
export function OpenRootInMenu({ root }: { root: FilesRootRef }) {
  if (!hasOpenInBridge()) return null;
  return <OpenRootMenu root={root} />;
}

function OpenRootMenu({ root }: { root: FilesRootRef }) {
  const openIn = useOpenIn(root);
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
        <OpenInMenuItems openIn={openIn} request={{ path: "", directory: true }} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
