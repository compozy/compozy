import { tidyMenuTree, type MenuTreeNode } from "@compozy/ui";
import { AppWindow, Code2, Copy, FolderOpen, RefreshCw, Settings2 } from "@compozy/ui/icons";

import type { OpenTarget } from "@/systems/os";

import type { OpenIn, OpenInRequest } from "../hooks/use-open-in";
import { copyFilePath } from "./copy-file-path";
import { openTargetActionLabel } from "./open-in-targets";

export interface FileMenuInput {
  /** The desktop Open in targets; null where this client cannot open them (a browser tab). */
  openIn: OpenIn | null;
  request: OpenInRequest;
  /** The root-relative path Copy path writes; omitted where there is no path to copy. */
  copyPath?: string;
  /** The surface's own open verbs, ahead of the editors (Changes: Open in Files). */
  leading?: readonly MenuTreeNode[];
}

/**
 * The one file menu: the configured editor and the reveal target one click away, the other
 * detected editors and the list's upkeep behind Open in ▸, then Copy path. The tree row, the
 * viewer's split button, the tree root header and a Changes file header all render this tree
 * (US-006.AC-2, AC-4, AC-5).
 */
export function fileMenuNodes({
  openIn,
  request,
  copyPath,
  leading = [],
}: FileMenuInput): MenuTreeNode[] {
  return tidyMenuTree([
    ...leading,
    ...(openIn ? openInNodes(openIn, request) : []),
    { kind: "separator", id: "copy" },
    ...(copyPath === undefined
      ? []
      : [
          {
            kind: "item",
            id: "copy-path",
            label: "Copy path",
            icon: Copy,
            onSelect: () => copyFilePath(copyPath),
          } as const,
        ]),
  ]);
}

function openInNodes(openIn: OpenIn, request: OpenInRequest): MenuTreeNode[] {
  const editors = openIn.targets.filter(target => target.kind === "editor");
  // `desktop.editor_command` when set, else the first detected editor: a lead that stays put.
  const lead = editors.find(target => target.id === "configured") ?? editors[0];
  const targetNode = (target: OpenTarget, label: string): MenuTreeNode => ({
    kind: "item",
    id: `target-${target.id}`,
    label,
    icon: target.kind === "reveal" ? FolderOpen : Code2,
    disabled: openIn.busy,
    onSelect: () => void openIn.open(target, request),
  });
  return [
    lead
      ? targetNode(lead, openTargetActionLabel(lead))
      : {
          kind: "item",
          id: "set-editor",
          label: "Set an editor in Settings",
          icon: Settings2,
          onSelect: openIn.openSettings,
        },
    {
      kind: "sub",
      id: "open-in",
      label: "Open in",
      icon: AppWindow,
      children: [
        ...editors.flatMap(target => (target === lead ? [] : [targetNode(target, target.label)])),
        { kind: "separator", id: "upkeep" },
        {
          kind: "item",
          id: "refresh",
          label: "Refresh list",
          icon: RefreshCw,
          onSelect: openIn.refresh,
        },
        {
          kind: "item",
          id: "settings",
          label: "Editor settings…",
          icon: Settings2,
          onSelect: openIn.openSettings,
        },
      ],
    },
    ...openIn.targets.flatMap(target =>
      target.kind === "reveal" ? [targetNode(target, openTargetActionLabel(target))] : []
    ),
  ];
}
