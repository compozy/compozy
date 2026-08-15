import { useState } from "react";
import { Plus } from "lucide-react";

import { cn } from "@compozy/ui";

import {
  WORKSPACES_MENU_CREATE_KEY,
  type OsWorkspacesWorktreeMenuModel,
} from "../lib/workspaces-overview-model";
import { OsWorkspacesWorktreeRow } from "./os-workspaces-worktree-row";
import type { WorktreeNestEntry } from "@/systems/workspace";

const MENU_GLASS_CLASS = cn(
  "w-[340px] max-w-[calc(100vw-48px)] rounded-window border border-line-strong",
  "bg-shell-glass-pop shadow-shell-strip backdrop-blur-shell-menu backdrop-saturate-[1.25]"
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
}: OsWorkspacesWorktreeMenuProps) {
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
    <div
      role="listbox"
      aria-label={`Worktrees of ${model.node.workspace.name}`}
      data-slot="os-workspaces-worktree-menu"
      data-testid="os-workspaces-worktree-menu"
      // Keyed by workspace so the entry animation replays per focus change.
      key={model.node.workspace.id}
      className={cn(MENU_GLASS_CLASS, "mt-1 p-[5px]", !reducedMotion && "os-wsov-menu-in")}
    >
      {model.visible.map(entry => {
        const navIndex = navIndexByKey.get(entry.key);
        return (
          <OsWorkspacesWorktreeRow
            key={entry.key}
            entry={entry}
            userHomeDir={userHomeDir}
            scoped={entry.key === scopedRowKey}
            focused={entry.key === focusedRowKey}
            actionsOpen={actionsKey === entry.key}
            onActionsOpenChange={open => setActionsKey(open ? entry.key : null)}
            onDelete={
              // Delete stays gated to adopted, ready records — the remove
              // flow's production gate; discovered rows have no record.
              entry.worktree && entry.displayState === "ready"
                ? () => onDeleteRow(entry)
                : undefined
            }
            registerRow={registerRow(entry.key)}
            onSelect={navIndex === undefined ? undefined : rowHandlers(navIndex).onClick}
          />
        );
      })}
      {canCreate ? (
        <>
          <div aria-hidden="true" className="mx-1.5 my-1 h-px bg-line-soft" />
          <div
            ref={registerRow(WORKSPACES_MENU_CREATE_KEY)}
            role="option"
            aria-selected={false}
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
            onClick={createNavIndex === undefined ? undefined : rowHandlers(createNavIndex).onClick}
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
    </div>
  );
}
