import { Ellipsis, Plus } from "lucide-react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  MenubarItem,
  MenubarRadioGroup,
  MenubarRadioItem,
  MenubarSeparator,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  cn,
} from "@compozy/ui";

import type { WorkspaceTreeNode } from "../lib/workspace-tree";
import { canRemoveWorktree, type WorktreeNestEntry } from "../lib/worktree-display";
import { copyWorktreePath } from "../lib/copy-worktree-path";
import { WorktreeNest } from "./worktree-nest";
import { WorktreeNestRow } from "./worktree-nest-row";

/** One-line action rows keep the nest rhythm beside two-line worktree rows. */
const FOOTER_ROW_CLASS = "min-h-control-compact";

/** Locked nest frame. Paths truncate inside; they never size the popup. */
export const WORKTREE_SUBMENU_FRAME_CLASS =
  "w-(--width-worktree-submenu) max-w-[calc(100vw-2rem)] overflow-x-hidden";

/** Panel-variant scroll caps: frame minus the pinned create footer. */
const PANEL_FRAME_MAX_CLASS = "max-h-[min(var(--height-worktree-submenu-max),calc(100dvh-2rem))]";
const PANEL_VIEWPORT_MAX_CLASS =
  "max-h-[calc(min(var(--height-worktree-submenu-max),calc(100dvh-2rem))-2.5rem)]";
/** Menu variant renders under the menubar; the cap keeps the popup on screen. */
const MENU_VIEWPORT_MAX_CLASS = "max-h-[min(var(--height-worktree-submenu-max),calc(100dvh-8rem))]";

export interface WorktreeSubmenuPanelProps {
  node: WorkspaceTreeNode<{ id: string; name: string }>;
  selectedWorktreeId?: string | null;
  testIdPrefix: string;
  /** Menu items for S2; buttons + dropdown actions for S1/S3 popovers. */
  variant?: "menu" | "panel";
  onSelectWorktree?: (entry: WorktreeNestEntry) => void;
  onCreateWorktree?: () => void;
  onRemoveWorktree?: (entry: WorktreeNestEntry) => void;
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  onOpenContext?: (entry: WorktreeNestEntry) => void;
  onClose?: () => void;
}

function WorktreeSubmenuRowActions({
  entry,
  testIdPrefix,
  variant,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
}: {
  entry: WorktreeNestEntry;
  testIdPrefix: string;
  variant: "menu" | "panel";
  onRemoveWorktree?: (entry: WorktreeNestEntry) => void;
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  onOpenContext?: (entry: WorktreeNestEntry) => void;
}) {
  const canRemove = Boolean(onRemoveWorktree) && canRemoveWorktree(entry);
  const canResolve = Boolean(onResolveMissing) && entry.displayState === "missing";
  const canOpenContext = Boolean(onOpenContext) && entry.displayState === "ready";
  const triggerClass =
    "inline-flex size-6 shrink-0 items-center justify-center rounded-md p-0 text-faint hover:bg-row-hover hover:text-fg";
  const items = (
    <>
      <ActionItem
        testId={`${testIdPrefix}-worktree-copy-path-${entry.key}`}
        variant={variant}
        onSelect={() => copyWorktreePath(entry.path)}
      >
        Copy path
      </ActionItem>
      {canOpenContext ? (
        <ActionItem
          testId={`${testIdPrefix}-worktree-context-${entry.key}`}
          variant={variant}
          onSelect={() => onOpenContext?.(entry)}
        >
          Context…
        </ActionItem>
      ) : null}
      {canResolve ? (
        <ActionItem
          testId={`${testIdPrefix}-worktree-resolve-${entry.key}`}
          variant={variant}
          onSelect={() => onResolveMissing?.(entry)}
        >
          Resolve…
        </ActionItem>
      ) : null}
      {canRemove ? (
        <ActionItem
          testId={`${testIdPrefix}-worktree-remove-${entry.key}`}
          variant={variant}
          destructive
          onSelect={() => onRemoveWorktree?.(entry)}
        >
          Remove…
        </ActionItem>
      ) : null}
    </>
  );

  if (variant === "menu") {
    return (
      <MenubarSub>
        <MenubarSubTrigger
          openOnHover={false}
          aria-label={`Actions for ${entry.name}`}
          data-testid={`${testIdPrefix}-worktree-actions-${entry.key}`}
          className={cn(triggerClass, "[&>svg:last-child]:hidden")}
        >
          <Ellipsis className="size-3.5" />
        </MenubarSubTrigger>
        <MenubarSubContent>{items}</MenubarSubContent>
      </MenubarSub>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`Actions for ${entry.name}`}
        data-testid={`${testIdPrefix}-worktree-actions-${entry.key}`}
        className={triggerClass}
      >
        <Ellipsis className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" side="right" className="min-w-36">
        {items}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ActionItem({
  testId,
  variant,
  destructive = false,
  onSelect,
  children,
}: {
  testId: string;
  variant: "menu" | "panel";
  destructive?: boolean;
  onSelect: () => void;
  children: React.ReactNode;
}) {
  if (variant === "menu") {
    return (
      <MenubarItem
        data-testid={testId}
        variant={destructive ? "destructive" : "default"}
        onClick={onSelect}
      >
        {children}
      </MenubarItem>
    );
  }
  return (
    <DropdownMenuItem
      data-testid={testId}
      variant={destructive ? "destructive" : "default"}
      onClick={onSelect}
    >
      {children}
    </DropdownMenuItem>
  );
}

/**
 * Locked worktree nest for every host submenu. Every worktree of the
 * workspace stays in the list — long nests scroll inside the shared frame,
 * never fold behind an overflow jump. Hosts only wrap this panel
 * (MenubarSub vs hover Popover).
 */
export function WorktreeSubmenuPanel({
  node,
  selectedWorktreeId,
  testIdPrefix,
  variant = "panel",
  onSelectWorktree,
  onCreateWorktree,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
  onClose,
}: WorktreeSubmenuPanelProps) {
  const workspaceId = node.workspace.id;
  const hasListRows = node.worktrees.length > 0;
  const selectEntry = (entry: WorktreeNestEntry) => {
    if (!entry.selectable) return;
    onSelectWorktree?.(entry);
    onClose?.();
  };

  const rows = node.worktrees.map(entry => {
    const checked = entry.worktree != null && entry.worktree.id === selectedWorktreeId;
    const actions = (
      <WorktreeSubmenuRowActions
        entry={entry}
        testIdPrefix={testIdPrefix}
        variant={variant}
        onRemoveWorktree={onRemoveWorktree}
        onResolveMissing={onResolveMissing}
        onOpenContext={onOpenContext}
      />
    );
    const body = <WorktreeNestRow entry={entry} checked={checked} />;
    if (variant === "menu") {
      return (
        <div
          key={entry.key}
          className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-0.5"
        >
          <MenubarRadioItem
            value={entry.worktree?.id ?? entry.key}
            disabled={!entry.selectable}
            closeOnClick
            className="group/wtnest min-w-0 overflow-hidden pr-1.5 *:data-[slot=dropdown-menu-radio-item-indicator]:hidden"
            data-testid={`${testIdPrefix}-worktree-option-${entry.key}`}
            onClick={() => selectEntry(entry)}
          >
            {body}
          </MenubarRadioItem>
          <span className="flex size-6 shrink-0 items-center justify-center">{actions}</span>
        </div>
      );
    }
    return (
      <div
        key={entry.key}
        className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-0.5"
      >
        <button
          type="button"
          aria-pressed={checked}
          disabled={!entry.selectable}
          data-testid={`${testIdPrefix}-worktree-option-${entry.key}`}
          className={cn(
            "group/wtnest flex min-h-control-compact min-w-0 items-center overflow-hidden rounded-md px-1.5 py-1 text-left",
            "hover:bg-elevated focus-visible:bg-elevated focus-visible:outline-none focus-visible:shadow-focus-ring",
            !entry.selectable && "pointer-events-none opacity-50"
          )}
          onClick={() => selectEntry(entry)}
        >
          {body}
        </button>
        <span className="flex size-6 shrink-0 items-center justify-center">{actions}</span>
      </div>
    );
  });

  const create = onCreateWorktree ? (
    variant === "menu" ? (
      <>
        {hasListRows ? <MenubarSeparator /> : null}
        <MenubarItem
          className={FOOTER_ROW_CLASS}
          data-testid={`${testIdPrefix}-worktree-create-${workspaceId}`}
          onClick={() => {
            onCreateWorktree();
            onClose?.();
          }}
        >
          <Plus className="size-3 text-faint" />
          New worktree
        </MenubarItem>
      </>
    ) : (
      <>
        {hasListRows ? <div className="my-1 h-px bg-line" /> : null}
        <button
          type="button"
          data-testid={`${testIdPrefix}-worktree-create-${workspaceId}`}
          className={cn(
            FOOTER_ROW_CLASS,
            "flex w-full items-center gap-2 rounded-md px-1.5 text-form-label text-subtle hover:bg-elevated"
          )}
          onClick={() => {
            onCreateWorktree();
            onClose?.();
          }}
        >
          <Plus className="size-3 text-faint" />
          New worktree
        </button>
      </>
    )
  ) : null;

  if (variant === "menu") {
    return (
      <WorktreeNest viewportClassName={MENU_VIEWPORT_MAX_CLASS} footer={create}>
        <MenubarRadioGroup
          value={selectedWorktreeId ?? ""}
          aria-label={`Worktrees in ${node.workspace.name}`}
        >
          {rows}
        </MenubarRadioGroup>
      </WorktreeNest>
    );
  }

  return (
    <WorktreeNest
      role="group"
      aria-label={`Worktrees in ${node.workspace.name}`}
      className={PANEL_FRAME_MAX_CLASS}
      viewportClassName={PANEL_VIEWPORT_MAX_CLASS}
      listClassName="pr-2"
      footer={create}
    >
      {rows}
    </WorktreeNest>
  );
}
