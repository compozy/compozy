import {
  useWorktreeRemovalSelection,
  WorktreeSelectionToolbar,
  type WorktreeRemovalBatch,
  type WorktreeRemovalProfile,
} from "@/systems/workspace";
import { useState } from "react";
import { Plus } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import {
  WORKSPACES_MENU_CREATE_KEY,
  type OsWorkspacesWorktreeMenuModel,
} from "../lib/workspaces-overview-model";
import { OsWorkspacesWorktreeRow } from "./os-workspaces-worktree-row";
import {
  canRemoveWorktree,
  WorktreeNest,
  WorktreeNestRow,
  type WorktreeNestEntry,
} from "@/systems/workspace";

const MENU_GLASS_CLASS = cn(
  "w-workspaces-menu max-w-workspaces-menu-max rounded-window border border-line-strong",
  "bg-shell-glass-pop shadow-shell-strip backdrop-blur-shell-menu backdrop-saturate-shell-glass"
);

export interface OsWorkspacesWorktreeMenuProps {
  model: OsWorkspacesWorktreeMenuModel;
  /** Display-contracts home-rooted row paths (`~`); actions keep absolute paths. */
  userHomeDir?: string;
  /** Menu-layer roving focus target, `null` while the strip owns focus. */
  focusedRowKey: string | null;
  scopedRowKey: string | null;
  reducedMotion: boolean;
  registerRow: (key: string) => (element: HTMLElement | null) => void;
  rowHandlers: (navIndex: number) => { onClick: () => void };
  canCreate: boolean;
  removalProfile?: WorktreeRemovalProfile | null;
  onRemoveWorktrees?: (batch: WorktreeRemovalBatch) => void;
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  onDeleteRow: (entry: WorktreeNestEntry) => void;
}

/**
 * The focused git-backed workspace's worktrees as a quiet vertical menu —
 * always visible, no disclosure gesture, never a horizontal strip. Git-backed
 * with zero worktrees renders only the lone dashed create button.
 */
export function OsWorkspacesWorktreeMenu({
  model,
  userHomeDir,
  focusedRowKey,
  scopedRowKey,
  reducedMotion,
  registerRow,
  rowHandlers,
  canCreate,
  onDeleteRow,
  removalProfile,
  onRemoveWorktrees,
  onResolveMissing,
}: OsWorkspacesWorktreeMenuProps) {
  const selection = useWorktreeRemovalSelection(
    model.node.workspace.id,
    model.visible,
    removalProfile,
    onRemoveWorktrees
  );
  // One popover at a time by construction: a single open-key owns them all.
  const [actionsKey, setActionsKey] = useState<string | null>(null);
  const navIndexByKey = new Map(model.navRows.map((row, index) => [row.key, index]));
  const createNavIndex = navIndexByKey.get(WORKSPACES_MENU_CREATE_KEY);

  if (model.visible.length === 0) {
    if (!canCreate) return null;
    return (
      <button
        type="button"
        data-testid="os-workspaces-worktree-new"
        ref={registerRow(WORKSPACES_MENU_CREATE_KEY)}
        tabIndex={focusedRowKey === WORKSPACES_MENU_CREATE_KEY ? 0 : -1}
        className={cn(
          "mt-2 inline-flex h-7 items-center gap-1.5 rounded-md border border-dashed border-line-strong px-3",
          "text-form-label font-medium whitespace-nowrap text-muted",
          "transition-colors duration-base ease-out",
          "hover:bg-btn-default-fill hover:text-fg-strong",
          "focus-visible:bg-btn-default-fill focus-visible:text-fg-strong focus-visible:outline-none",
          !reducedMotion && "os-wsov-menu-in"
        )}
        onClick={createNavIndex === undefined ? undefined : rowHandlers(createNavIndex).onClick}
      >
        <Plus aria-hidden="true" className="size-3" />
        New worktree
      </button>
    );
  }

  return (
    <WorktreeNest
      onKeyDown={event => {
        selection.onKeyDown(event);
        if (!selection.mode || event.defaultPrevented) return;
        // Selection owns keyboard focus, including missing rows that navigation excludes.
        event.stopPropagation();
        if (!["ArrowDown", "ArrowUp", "Home", "End", "Tab"].includes(event.key)) return;
        const controls = Array.from(
          event.currentTarget.querySelectorAll<HTMLElement>(
            'button:not(:disabled), [role="menuitem"][tabindex="0"]'
          )
        );
        if (!controls.length) return;
        event.preventDefault();
        const index = controls.indexOf(document.activeElement as HTMLElement);
        const direction =
          event.key === "ArrowUp" || (event.key === "Tab" && event.shiftKey) ? -1 : 1;
        const next =
          event.key === "Home"
            ? 0
            : event.key === "End"
              ? controls.length - 1
              : (index + direction + controls.length) % controls.length;
        controls[next]?.focus();
      }}
      role="menu"
      aria-label={`Worktrees of ${model.node.workspace.name}`}
      data-slot="os-workspaces-worktree-menu"
      data-testid="os-workspaces-worktree-menu"
      className={cn(
        MENU_GLASS_CLASS,
        "mt-1 p-workspaces-menu-gap",
        !reducedMotion && "os-wsov-menu-in"
      )}
      viewportClassName="max-h-workspaces-menu-max"
      footer={
        <>
          <WorktreeSelectionToolbar selection={selection} />
          {canCreate ? (
            <>
              <hr className="mx-1.5 my-1 h-px border-0 bg-line-soft" />
              <div
                ref={registerRow(WORKSPACES_MENU_CREATE_KEY)}
                role="menuitem"
                aria-label="New worktree"
                tabIndex={focusedRowKey === WORKSPACES_MENU_CREATE_KEY ? 0 : -1}
                data-testid="os-workspaces-worktree-create"
                data-on={focusedRowKey === WORKSPACES_MENU_CREATE_KEY ? "true" : undefined}
                className={cn(
                  "group/wsov-foot grid min-h-7.5 w-full grid-cols-[16px_minmax(0,1fr)] items-center gap-2 rounded-md px-2 py-1",
                  "text-left outline-none select-none",
                  "transition-colors duration-base ease-out hover:bg-row-hover",
                  "focus-visible:outline-none",
                  focusedRowKey === WORKSPACES_MENU_CREATE_KEY && "bg-row-selected"
                )}
                onClick={
                  createNavIndex === undefined ? undefined : rowHandlers(createNavIndex).onClick
                }
              >
                <Plus aria-hidden="true" className="size-3 justify-self-center text-subtle" />
                <b
                  className={cn(
                    "truncate text-form-label font-medium text-muted",
                    "group-hover/wsov-foot:text-fg-strong",
                    focusedRowKey === WORKSPACES_MENU_CREATE_KEY && "text-fg-strong"
                  )}
                >
                  New worktree
                </b>
              </div>
            </>
          ) : null}
        </>
      }
    >
      {model.visible.map(entry => {
        const navIndex = navIndexByKey.get(entry.key);
        if (selection.mode) {
          const reason = selection.reason(entry);
          return (
            <Button
              key={entry.key}
              variant="ghost"
              role="menuitemcheckbox"
              aria-checked={selection.selectedIds.has(entry.key)}
              aria-label={`Select ${entry.name}`}
              disabled={Boolean(reason)}
              title={reason ?? undefined}
              className="group/wtnest h-auto w-full flex-col items-stretch px-2 py-1.5 text-left"
              onClick={event => selection.toggle(entry, event.shiftKey)}
              onKeyDown={event => {
                if (event.key === "Enter" || event.key === " ") event.stopPropagation();
              }}
            >
              <WorktreeNestRow
                entry={{ ...entry, adoptable: false, inertReason: reason }}
                userHomeDir={userHomeDir}
                checked={selection.selectedIds.has(entry.key)}
              />
            </Button>
          );
        }
        return (
          <OsWorkspacesWorktreeRow
            key={entry.key}
            entry={entry}
            onResolveMissing={
              entry.displayState === "missing" &&
              onResolveMissing &&
              (!removalProfile || !selection.reason(entry))
                ? () => onResolveMissing(entry)
                : undefined
            }
            userHomeDir={userHomeDir}
            scoped={entry.key === scopedRowKey}
            focused={entry.key === focusedRowKey}
            actionsOpen={actionsKey === entry.key}
            onActionsOpenChange={open => setActionsKey(open ? entry.key : null)}
            onDelete={
              // Delete stays gated to adopted, ready records — the remove
              // flow's production gate; discovered rows have no record.
              canRemoveWorktree(entry) && (!removalProfile || !selection.reason(entry))
                ? () => onDeleteRow(entry)
                : undefined
            }
            registerRow={registerRow(entry.key)}
            onSelect={navIndex === undefined ? undefined : rowHandlers(navIndex).onClick}
          />
        );
      })}
    </WorktreeNest>
  );
}
