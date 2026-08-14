import { Check, Ellipsis, Plus } from "lucide-react";
import { toast } from "sonner";

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
  ScrollArea,
  cn,
} from "@compozy/ui";

import type { WorkspaceTreeNode } from "../lib/workspace-tree";
import type { WorktreeNestEntry } from "../lib/worktree-display";
import { truncateWorktreeNest } from "../lib/worktree-sort";
import { WorktreeRow } from "./worktree-row";

/** One-line action rows keep the nest rhythm beside two-line worktree rows. */
const FOOTER_ROW_CLASS = "min-h-control-compact";

/** Locked nest frame. Paths truncate inside; they never size the popup. */
export const WORKTREE_SUBMENU_FRAME_CLASS =
  "w-(--width-worktree-submenu) max-w-[calc(100vw-2rem)] overflow-x-hidden";

export interface WorktreeSubmenuPanelProps {
  node: WorkspaceTreeNode<{ id: string; name: string }>;
  selectedWorktreeId?: string | null;
  testIdPrefix: string;
  /** Menu items for S2; buttons + dropdown actions for S1/S3 popovers. */
  variant?: "menu" | "panel";
  onSelectWorktree?: (entry: WorktreeNestEntry) => void;
  onCreateWorktree?: () => void;
  onShowAllWorktrees?: () => void;
  onRemoveWorktree?: (entry: WorktreeNestEntry) => void;
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  onOpenContext?: (entry: WorktreeNestEntry) => void;
  onClose?: () => void;
  limit?: number;
}

function copyWorktreePath(path: string) {
  navigator.clipboard.writeText(path).then(
    () => toast.success("Path copied"),
    () => toast.error("Could not copy the path")
  );
}

function WorktreeSubmenuTrailing({
  entry,
  checked,
}: {
  entry: WorktreeNestEntry;
  checked: boolean;
}) {
  return (
    <>
      {entry.adoptable ? (
        <span className="text-mono-id font-semibold text-info opacity-0 transition-opacity duration-fast group-hover/worktree-item:opacity-100 group-focus/worktree-item:opacity-100">
          Adopt
        </span>
      ) : null}
      {checked ? <Check className="size-3 text-accent" /> : null}
    </>
  );
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
  const canRemove =
    Boolean(onRemoveWorktree) && entry.worktree != null && entry.displayState === "ready";
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

function WorktreeSubmenuRow({ entry, checked }: { entry: WorktreeNestEntry; checked: boolean }) {
  return (
    <WorktreeRow
      entry={entry}
      density="nest"
      className="w-full min-w-0 overflow-hidden px-0 hover:bg-transparent"
      trailing={<WorktreeSubmenuTrailing entry={entry} checked={checked} />}
    />
  );
}

/**
 * Locked worktree nest for every host submenu. Sort and truncation stay in the
 * shared lib; hosts only wrap this panel (MenubarSub vs hover Popover).
 */
export function WorktreeSubmenuPanel({
  node,
  selectedWorktreeId,
  testIdPrefix,
  variant = "panel",
  onSelectWorktree,
  onCreateWorktree,
  onShowAllWorktrees,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
  onClose,
  limit,
}: WorktreeSubmenuPanelProps) {
  const nest = truncateWorktreeNest(node.worktrees, limit);
  const workspaceId = node.workspace.id;
  const hasListRows = nest.visible.length > 0 || nest.overflowCount > 0;
  const selectEntry = (entry: WorktreeNestEntry) => {
    if (!entry.selectable) return;
    onSelectWorktree?.(entry);
    onClose?.();
  };

  const rows = nest.visible.map(entry => {
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
            className="group/worktree-item min-w-0 overflow-hidden pr-1.5 *:data-[slot=dropdown-menu-radio-item-indicator]:hidden"
            data-testid={`${testIdPrefix}-worktree-option-${entry.key}`}
            onClick={() => selectEntry(entry)}
          >
            <WorktreeSubmenuRow entry={entry} checked={checked} />
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
            "group/worktree-item flex min-h-control-compact min-w-0 items-center overflow-hidden rounded-md px-1.5 py-1 text-left",
            "hover:bg-elevated focus-visible:bg-elevated focus-visible:outline-none focus-visible:shadow-focus-ring",
            !entry.selectable && "pointer-events-none opacity-50"
          )}
          onClick={() => selectEntry(entry)}
        >
          <WorktreeSubmenuRow entry={entry} checked={checked} />
        </button>
        <span className="flex size-6 shrink-0 items-center justify-center">{actions}</span>
      </div>
    );
  });

  const overflow =
    nest.overflowCount > 0 && onShowAllWorktrees ? (
      variant === "menu" ? (
        <MenubarItem
          className={FOOTER_ROW_CLASS}
          data-testid={`${testIdPrefix}-worktree-overflow-${workspaceId}`}
          onClick={() => {
            onShowAllWorktrees();
            onClose?.();
          }}
        >
          <Ellipsis className="size-3 text-faint" />
          All {node.adoptedCount} worktrees
        </MenubarItem>
      ) : (
        <button
          type="button"
          data-testid={`${testIdPrefix}-worktree-overflow-${workspaceId}`}
          className={cn(
            FOOTER_ROW_CLASS,
            "flex w-full items-center gap-2 rounded-md px-1.5 text-form-label text-subtle hover:bg-elevated"
          )}
          onClick={() => {
            onShowAllWorktrees();
            onClose?.();
          }}
        >
          <Ellipsis className="size-3 text-faint" />
          All {node.adoptedCount} worktrees
        </button>
      )
    ) : null;

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
      <>
        <MenubarRadioGroup
          value={selectedWorktreeId ?? ""}
          aria-label={`Worktrees in ${node.workspace.name}`}
        >
          {rows}
        </MenubarRadioGroup>
        {overflow}
        {create}
      </>
    );
  }

  return (
    <div
      role="group"
      aria-label={`Worktrees in ${node.workspace.name}`}
      className="flex max-h-[min(var(--height-worktree-submenu-max),calc(100dvh-2rem))] flex-col"
    >
      <ScrollArea
        className="min-h-0"
        viewportClassName="max-h-[calc(min(var(--height-worktree-submenu-max),calc(100dvh-2rem))-2.5rem)]"
      >
        <div className="pr-2">{rows}</div>
      </ScrollArea>
      {overflow}
      {create}
    </div>
  );
}
