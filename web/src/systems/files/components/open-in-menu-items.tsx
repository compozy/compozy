import { DropdownMenuItem, DropdownMenuSeparator } from "@compozy/ui";
import { Code2, Copy, FolderOpen, RefreshCw, Settings } from "@compozy/ui/icons";
import { CursorLogo } from "@compozy/ui/logos";

import type { OpenTarget } from "@/systems/os";

import type { OpenIn, OpenInRequest } from "../hooks/use-open-in";
import { copyFilePath } from "../lib/copy-file-path";

/** Brand marks that exist in `@compozy/ui/logos`; other editors use the generic code icon. */
export function OpenTargetIcon({ target }: { target: OpenTarget }) {
  if (target.kind === "reveal") return <FolderOpen aria-hidden />;
  if (target.id === "cursor") return <CursorLogo aria-hidden className="size-4" />;
  return <Code2 aria-hidden />;
}

/**
 * The Open in menu (`_uiux.md` S5): the configured editor, the detected editors, the file
 * manager and Copy path, then Refresh list and Editor settings. Shared by the viewer's split
 * button, the tree row menu, and the tree root header (US-006.AC-2, AC-4, AC-5).
 */
export function OpenInMenuItems({
  openIn,
  request,
  copyPath,
}: {
  openIn: OpenIn;
  request: OpenInRequest;
  /** The root-relative path Copy path writes; omitted where the menu has its own Copy path. */
  copyPath?: string;
}) {
  const configured = openIn.targets.filter(target => target.id === "configured");
  const detected = openIn.targets.filter(
    target => target.kind === "editor" && target.id !== "configured"
  );
  const reveal = openIn.targets.filter(target => target.kind === "reveal");
  const item = (target: OpenTarget) => (
    <DropdownMenuItem
      key={target.id}
      disabled={openIn.busy}
      onClick={() => void openIn.open(target, request)}
    >
      <OpenTargetIcon target={target} />
      {target.label}
    </DropdownMenuItem>
  );
  return (
    <>
      {configured.map(item)}
      {configured.length > 0 && detected.length > 0 ? <DropdownMenuSeparator /> : null}
      {detected.map(item)}
      {configured.length + detected.length === 0 ? (
        <DropdownMenuItem onClick={openIn.openSettings}>
          <Settings aria-hidden />
          Set an editor in Settings
        </DropdownMenuItem>
      ) : null}
      <DropdownMenuSeparator />
      {reveal.map(item)}
      {copyPath !== undefined ? (
        <DropdownMenuItem onClick={() => copyFilePath(copyPath)}>
          <Copy aria-hidden />
          Copy path
        </DropdownMenuItem>
      ) : null}
      <DropdownMenuSeparator />
      <DropdownMenuItem onClick={openIn.refresh}>
        <RefreshCw aria-hidden />
        Refresh list
      </DropdownMenuItem>
      <DropdownMenuItem onClick={openIn.openSettings}>
        <Settings aria-hidden />
        Editor settings…
      </DropdownMenuItem>
    </>
  );
}
