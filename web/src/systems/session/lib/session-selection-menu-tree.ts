/**
 * The selection's menu: the session menu's tree minus the verbs that mean something for one
 * session only (Rename, New from this, linked work, Clear conversation, manual order). The
 * selection bar's More actions and a selected row's or card's right-click render it. Counts ride
 * as hints, and a verb no selected session can take is absent.
 */
import {
  Clipboard,
  CornerDownRight,
  Folder,
  FolderInput,
  FolderOutput,
  FolderPlus,
  GitBranch,
} from "@compozy/ui/icons";

import { tidyMenuTree, type MenuTreeHost, type MenuTreeNode } from "@compozy/ui";

import { getSessionDisplayTitle } from "./session-display-title";
import { sessionCountLabel } from "./session-folders";
import { sessionBatchAction, type SessionBatchMenuActionId } from "./session-menu-actions";
import type { SessionBulkFolderMenu, SessionSelectionVerbs } from "./session-selection-verbs";

type ItemNode = Extract<MenuTreeNode, { kind: "item" }>;

const separator = (id: string): MenuTreeNode => ({ kind: "separator", id });

/** Test ids read `<testId>-<suffix>`; these verbs keep their established suffixes. */
const TEST_SUFFIX: Partial<Record<SessionBatchMenuActionId, string>> = {
  "copy-id": "copy-ids",
  "copy-title": "copy-titles",
  delete: "delete-item",
};

function folderGroup(menu: SessionBulkFolderMenu, testId: string): MenuTreeNode {
  const move = `${testId}-move`;
  if (menu.blockedReason !== null) {
    return {
      kind: "group",
      id: "folder",
      label: "Folder",
      children: [
        // Focusable but inert, unlike a disabled item: keyboard and screen reader users land on
        // the reason instead of an empty submenu, and choosing it keeps the menu open.
        {
          kind: "item",
          id: "folder-blocked",
          label: menu.blockedReason,
          closeOnClick: false,
          testId: `${move}-blocked`,
        },
      ],
    };
  }
  return {
    kind: "group",
    id: "folder",
    label: "Folder",
    children: [
      ...menu.folders.map<MenuTreeNode>(folder => {
        const current = folder.id === menu.sharedFolderId;
        return {
          kind: "item",
          id: `folder-${folder.id}`,
          label: folder.name,
          icon: Folder,
          checked: current,
          disabled: current,
          testId: `${move}-${folder.id}`,
          onSelect: () => menu.onMove(folder.id),
        };
      }),
      ...(menu.canUnfile
        ? [
            {
              kind: "item",
              id: "unfolder",
              label: "Remove from folder",
              icon: FolderOutput,
              testId: `${move}-root`,
              onSelect: () => menu.onMove(null),
            } as const,
          ]
        : []),
      {
        kind: "item",
        id: "new-folder",
        label: "New folder…",
        icon: FolderPlus,
        testId: `${move}-new`,
        onSelect: menu.onNewFolder,
      },
    ],
  };
}

/** The selection's verbs in menu order, without the count heading its host may add. */
export function selectionMenuTree(
  verbs: SessionSelectionVerbs,
  testId: string,
  host: MenuTreeHost
): MenuTreeNode[] {
  const { disabled } = verbs;
  const verb = (
    id: SessionBatchMenuActionId,
    count: number,
    onSelect: (() => void) | undefined,
    hint: string = sessionCountLabel(count)
  ): ItemNode[] => {
    // A verb no selected session can take is absent, not a disabled "0 sessions" row.
    if (count === 0 || !onSelect) return [];
    const action = sessionBatchAction(id, verbs.archivableLive > 0);
    return [
      {
        kind: "item",
        id,
        label: action.label,
        icon: action.icon,
        hint,
        disabled,
        testId: `${testId}-${TEST_SUFFIX[id] ?? id}`,
        onSelect,
      },
    ];
  };
  const copy = (id: "copy-id" | "copy-title", onSelect: () => void): ItemNode => {
    const action = sessionBatchAction(id);
    return {
      kind: "item",
      id,
      label: action.label,
      icon: action.icon,
      testId: `${testId}-${TEST_SUFFIX[id]}`,
      onSelect,
    };
  };
  const { moveUnder } = verbs;
  return tidyMenuTree([
    {
      kind: "sub",
      id: "copy",
      label: "Copy",
      icon: Clipboard,
      testId: `${testId}-copy`,
      children: [
        copy("copy-id", verbs.onCopyIds),
        copy("copy-title", verbs.onCopyTitles),
        separator("copy-work"),
        ...(verbs.branches > 0
          ? [
              {
                kind: "item",
                id: "copy-branch",
                label: "Copy branch names",
                icon: GitBranch,
                hint: sessionCountLabel(verbs.branches),
                testId: `${testId}-copy-branches`,
                onSelect: verbs.onCopyBranches,
              } as const,
            ]
          : []),
      ],
    },
    separator("organize"),
    {
      kind: "sub",
      id: "move",
      label: "Move",
      icon: FolderInput,
      disabled,
      testId: `${testId}-move`,
      children: [
        ...(verbs.folders ? [folderGroup(verbs.folders, testId)] : []),
        separator("move-thread"),
        // A session's own Move under… opens the picker; a selection picks from the listed rows.
        ...(moveUnder && moveUnder.parents.length > 0 && moveUnder.movable > 0
          ? [
              {
                kind: "sub",
                id: "move-under",
                label: "Move under",
                icon: CornerDownRight,
                hint: sessionCountLabel(moveUnder.movable),
                testId: `${testId}-move-under`,
                children: moveUnder.parents.map<MenuTreeNode>(parent => ({
                  kind: "item",
                  id: `move-under-${parent.id}`,
                  label: getSessionDisplayTitle(parent),
                  testId: `${testId}-move-under-${parent.id}`,
                  onSelect: () => moveUnder.onMoveUnder(parent),
                })),
              } as const,
            ]
          : []),
        ...verb("make-top-level", verbs.topLevelable, verbs.onMakeTopLevel),
      ],
    },
    ...verb("mark-seen", verbs.markSeenable, verbs.onMarkSeen),
    separator("lifecycle"),
    ...verb("stop", verbs.stoppable, verbs.onStop, `${verbs.stoppable} active`),
    ...verb("later", verbs.laterable, verbs.onMoveToLater),
    ...verb(
      "back-from-later",
      verbs.backFromLaterable,
      verbs.onBackFromLater,
      `${verbs.backFromLaterable} in Later`
    ),
    ...verb(
      "archive",
      verbs.archivable,
      verbs.onArchive,
      verbs.archivableLive === 0
        ? `${verbs.archivable} stopped`
        : sessionCountLabel(verbs.archivable)
    ),
    ...verb("unarchive", verbs.unarchivable, verbs.onUnarchive, `${verbs.unarchivable} archived`),
    separator("danger"),
    ...(verbs.onDelete
      ? [
          {
            kind: "item",
            id: "delete",
            label: `Delete ${sessionCountLabel(verbs.count)}`,
            icon: sessionBatchAction("delete").icon,
            // The list's ⌘⌫ cannot reach a portaled context menu: only the bar names it.
            ...(host === "dropdown" ? { hint: "⌘⌫" } : {}),
            destructive: true,
            disabled,
            testId: `${testId}-${TEST_SUFFIX.delete}`,
            onSelect: verbs.onDelete,
          } as const,
        ]
      : []),
  ]);
}
