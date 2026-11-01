import { Archive, Bookmark, Hash, RotateCcw, Square, Trash2, Undo2 } from "@compozy/ui/icons";

import {
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "@compozy/ui";

import type { SessionSelectionVerbs } from "../../hooks/use-session-list-bulk-selection";
import { sessionCountLabel } from "../../lib/session-folders";
import { SessionSelectionFolderMenu } from "./session-list-selection-folders";

/** The menu parts one selection menu renders with: the bar's dropdown or a row's context menu. */
const SELECTION_MENU_KITS = {
  dropdown: {
    Item: DropdownMenuItem,
    Separator: DropdownMenuSeparator,
    Shortcut: DropdownMenuShortcut,
    Sub: DropdownMenuSub,
    SubTrigger: DropdownMenuSubTrigger,
    SubContent: DropdownMenuSubContent,
  },
  context: {
    Item: ContextMenuItem,
    Separator: ContextMenuSeparator,
    Shortcut: ContextMenuShortcut,
    Sub: ContextMenuSub,
    SubTrigger: ContextMenuSubTrigger,
    SubContent: ContextMenuSubContent,
  },
} as const;

export type SessionSelectionMenuKind = keyof typeof SELECTION_MENU_KITS;
export type SessionSelectionMenuKit = (typeof SELECTION_MENU_KITS)[SessionSelectionMenuKind];

/** Archive stops live rows first, so the verb and count say so when the selection has any. */
function archiveSelectionCopy(archivable: number, archivableLive: number) {
  if (archivableLive === 0) return { label: "Archive", count: `${archivable} stopped` };
  return {
    label: "Stop and archive",
    count: `${archivable} ${archivable === 1 ? "session" : "sessions"}`,
  };
}

/**
 * The selection's verbs in menu order. The selection bar's More menu and a
 * selected row's context menu both render this, so the two never diverge.
 */
export function SessionSelectionMenuItems({
  verbs,
  kind,
  testId,
}: {
  verbs: SessionSelectionVerbs;
  kind: SessionSelectionMenuKind;
  /** Item test ids read `<testId>-<verb>`. */
  testId: string;
}) {
  const kit = SELECTION_MENU_KITS[kind];
  const { Item, Separator, Shortcut } = kit;
  const { disabled } = verbs;
  const archiveCopy = archiveSelectionCopy(verbs.archivable, verbs.archivableLive);
  return (
    <>
      {verbs.folders ? (
        <>
          <SessionSelectionFolderMenu
            kit={kit}
            menu={verbs.folders}
            disabled={disabled}
            testId={`${testId}-move`}
          />
          <Separator />
        </>
      ) : null}
      <Item
        disabled={disabled || verbs.stoppable === 0 || !verbs.onStop}
        onClick={verbs.onStop}
        data-testid={`${testId}-stop`}
      >
        <Square className="size-3" />
        Stop<Shortcut>{verbs.stoppable} active</Shortcut>
      </Item>
      {verbs.onMoveToLater ? (
        <Item
          disabled={disabled || verbs.laterable === 0}
          onClick={verbs.onMoveToLater}
          data-testid={`${testId}-later`}
        >
          <Bookmark className="size-3" />
          Move to Later…<Shortcut>{sessionCountLabel(verbs.laterable)}</Shortcut>
        </Item>
      ) : null}
      {verbs.onBackFromLater && verbs.backFromLaterable > 0 ? (
        <Item
          disabled={disabled}
          onClick={verbs.onBackFromLater}
          data-testid={`${testId}-back-from-later`}
        >
          <Undo2 className="size-3" />
          Back from Later<Shortcut>{verbs.backFromLaterable} in Later</Shortcut>
        </Item>
      ) : null}
      <Item
        disabled={disabled || verbs.archivable === 0 || !verbs.onArchive}
        onClick={verbs.onArchive}
        data-testid={`${testId}-archive`}
      >
        <Archive className="size-3" />
        {archiveCopy.label}
        <Shortcut>{archiveCopy.count}</Shortcut>
      </Item>
      <Item
        disabled={disabled || verbs.unarchivable === 0 || !verbs.onUnarchive}
        onClick={verbs.onUnarchive}
        data-testid={`${testId}-unarchive`}
      >
        <RotateCcw className="size-3" />
        Unarchive<Shortcut>{verbs.unarchivable} archived</Shortcut>
      </Item>
      <Item onClick={verbs.onCopyIds} data-testid={`${testId}-copy-ids`}>
        <Hash className="size-3" />
        Copy session IDs
      </Item>
      <Separator />
      <Item
        variant="destructive"
        disabled={disabled || !verbs.onDelete}
        onClick={verbs.onDelete}
        data-testid={`${testId}-delete-item`}
      >
        <Trash2 className="size-3" />
        Delete {sessionCountLabel(verbs.count)}
        {/* The bar's menu keeps the list's ⌘⌫ hint; a row's portaled context menu cannot route that key. */}
        {kind === "dropdown" ? <Shortcut>⌘⌫</Shortcut> : null}
      </Item>
    </>
  );
}

/**
 * A selected row's context menu inside a multi-selection: it names the
 * selection and offers its verbs, never the row's own.
 */
export function SessionSelectionContextItems({
  verbs,
  testId,
}: {
  verbs: SessionSelectionVerbs;
  testId: string;
}) {
  return (
    <ContextMenuGroup data-testid={testId}>
      <ContextMenuLabel data-testid={`${testId}-count`}>
        {sessionCountLabel(verbs.count)}
      </ContextMenuLabel>
      <SessionSelectionMenuItems verbs={verbs} kind="context" testId={testId} />
    </ContextMenuGroup>
  );
}
