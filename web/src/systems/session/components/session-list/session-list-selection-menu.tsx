import { Fragment } from "react";
import { sessionBatchAction, type SessionBatchMenuActionId } from "../../lib/session-menu-actions";
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

const SELECTION_TEST_SUFFIXES: Partial<Record<SessionBatchMenuActionId, string>> = {
  "copy-id": "copy-ids",
  "copy-title": "copy-titles",
  delete: "delete-item",
};

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
  const entries: Record<SessionBatchMenuActionId, { count: number; onClick?: () => void }> = {
    "mark-seen": { count: verbs.markSeenable, onClick: verbs.onMarkSeen },
    "make-top-level": { count: verbs.topLevelable, onClick: verbs.onMakeTopLevel },
    stop: { count: verbs.stoppable, onClick: verbs.onStop },
    later: { count: verbs.laterable, onClick: verbs.onMoveToLater },
    "back-from-later": { count: verbs.backFromLaterable, onClick: verbs.onBackFromLater },
    archive: { count: verbs.archivable, onClick: verbs.onArchive },
    unarchive: { count: verbs.unarchivable, onClick: verbs.onUnarchive },
    "copy-title": { count: verbs.count, onClick: verbs.onCopyTitles },
    "copy-id": { count: verbs.count, onClick: verbs.onCopyIds },
    delete: { count: verbs.count, onClick: verbs.onDelete },
  };
  const hints: Partial<Record<SessionBatchMenuActionId, string>> = {
    stop: `${verbs.stoppable} active`,
    archive:
      verbs.archivableLive === 0
        ? `${verbs.archivable} stopped`
        : sessionCountLabel(verbs.archivable),
    unarchive: `${verbs.unarchivable} archived`,
    "back-from-later": `${verbs.backFromLaterable} in Later`,
  };
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
      {(Object.keys(entries) as SessionBatchMenuActionId[]).map(id => {
        const entry = entries[id];
        const action = sessionBatchAction(id, verbs.archivableLive > 0);
        const Icon = action.icon;
        const copy = id === "copy-id" || id === "copy-title";
        if ((id === "later" || id === "back-from-later") && !entry.onClick) return null;
        if (id === "back-from-later" && entry.count === 0) return null;
        const suffix = SELECTION_TEST_SUFFIXES[id] ?? id;
        return (
          <Fragment key={id}>
            {id === "delete" ? <Separator /> : null}
            <Item
              variant={id === "delete" ? "destructive" : undefined}
              disabled={(!copy && disabled) || entry.count === 0 || !entry.onClick}
              onClick={entry.onClick}
              data-testid={`${testId}-${suffix}`}
            >
              <Icon className="size-3" />
              {id === "delete" ? `Delete ${sessionCountLabel(verbs.count)}` : action.label}
              {!copy && id !== "delete" ? (
                <Shortcut>{hints[id] ?? sessionCountLabel(entry.count)}</Shortcut>
              ) : null}
              {id === "delete" && kind === "dropdown" ? <Shortcut>⌘⌫</Shortcut> : null}
            </Item>
          </Fragment>
        );
      })}
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
