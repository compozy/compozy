import {
  useWorktreeRemovalSelection,
  type WorktreeRemovalBatch,
} from "../hooks/use-worktree-removal-selection";
import { WorktreeSelectionToolbar } from "./worktree-selection-toolbar";
import { Copy, Ellipsis, Eraser, FileSearch, Plus, Trash2 } from "@compozy/ui/icons";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  MenubarItem,
  MenubarCheckboxItem,
  MenubarRadioGroup,
  MenubarRadioItem,
  MenubarSeparator,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  cn,
} from "@compozy/ui";

import type { WorkspaceTreeNode } from "../lib/workspace-tree";
import {
  canRemoveWorktree,
  isStaleDiscovered,
  type WorktreeNestEntry,
} from "../lib/worktree-display";
import { copyWorktreePath } from "../lib/copy-worktree-path";
import { WorktreeNest } from "./worktree-nest";
import { WorktreeNestRow } from "./worktree-nest-row";

/** One-line action rows keep the nest rhythm beside two-line worktree rows. */
const FOOTER_ROW_CLASS = "min-h-control-compact";

/** Locked nest frame. Paths truncate inside; they never size the popup. */
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
  /** Display-contracts home-rooted nest paths; actions keep the absolute path. */
  userHomeDir?: string;
  onSelectWorktree?: (entry: WorktreeNestEntry) => void;
  onCreateWorktree?: () => void;
  onRemoveWorktree?: (entry: WorktreeNestEntry) => void;
  onResolveMissing?: (entry: WorktreeNestEntry) => void;
  onOpenContext?: (entry: WorktreeNestEntry) => void;
  onClose?: () => void;
  onRemoveWorktrees?: (batch: WorktreeRemovalBatch) => void;
}

/**
 * Exposes singular cleanup actions for the row's lifecycle state. A refused
 * cleanup stays visible as a disabled item that names the reason.
 */
function WorktreeSubmenuRowActions({
  entry,
  cleanupReason,
  testIdPrefix,
  variant,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
}: {
  entry: WorktreeNestEntry;
  cleanupReason: string | null;
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
    "inline-flex size-6 shrink-0 items-center justify-center rounded-md p-0 text-faint hover:bg-surface-2 hover:text-fg";
  const items = (
    <>
      <ActionItem
        testId={`${testIdPrefix}-worktree-copy-path-${entry.key}`}
        variant={variant}
        onSelect={() => copyWorktreePath(entry.path)}
        icon={Copy}
      >
        Copy path
      </ActionItem>
      {canOpenContext ? (
        <ActionItem
          testId={`${testIdPrefix}-worktree-context-${entry.key}`}
          variant={variant}
          onSelect={() => onOpenContext?.(entry)}
          icon={FileSearch}
        >
          Context…
        </ActionItem>
      ) : null}
      {canResolve ? (
        <ActionItem
          testId={`${testIdPrefix}-worktree-resolve-${entry.key}`}
          variant={variant}
          disabledReason={cleanupReason}
          onSelect={() => onResolveMissing?.(entry)}
          icon={Eraser}
        >
          Clean up missing record…
        </ActionItem>
      ) : null}
      {canRemove ? (
        <ActionItem
          testId={`${testIdPrefix}-worktree-remove-${entry.key}`}
          variant={variant}
          destructive
          disabledReason={cleanupReason}
          onSelect={() => onRemoveWorktree?.(entry)}
          icon={Trash2}
        >
          {entry.discovered && isStaleDiscovered(entry.discovered)
            ? "Remove stale entry…"
            : "Remove…"}
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
  disabledReason = null,
  icon: ItemIcon,
  onSelect,
  children,
}: {
  testId: string;
  variant: "menu" | "panel";
  destructive?: boolean;
  /** Keeps a refused action discoverable: disabled, with its reason on a second line. */
  disabledReason?: string | null;
  icon: typeof Copy;
  onSelect: () => void;
  children: React.ReactNode;
}) {
  const content = (
    <>
      <ItemIcon
        aria-hidden="true"
        className={cn(
          "size-deck-glyph",
          disabledReason && "mt-0.5 self-start",
          !destructive && "text-subtle"
        )}
      />
      {disabledReason ? (
        <span className="flex min-w-0 flex-col">
          <span>{children}</span>
          <span data-slot="worktree-action-reason" className="text-form-hint text-subtle">
            {disabledReason}
          </span>
        </span>
      ) : (
        children
      )}
    </>
  );
  if (variant === "menu") {
    return (
      <MenubarItem
        data-testid={testId}
        variant={destructive ? "destructive" : "default"}
        disabled={Boolean(disabledReason)}
        onClick={onSelect}
      >
        {content}
      </MenubarItem>
    );
  }
  return (
    <DropdownMenuItem
      data-testid={testId}
      variant={destructive ? "destructive" : "default"}
      disabled={Boolean(disabledReason)}
      onClick={onSelect}
    >
      {content}
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
  userHomeDir,
  onSelectWorktree,
  onCreateWorktree,
  onRemoveWorktree,
  onResolveMissing,
  onOpenContext,
  onClose,
  onRemoveWorktrees,
}: WorktreeSubmenuPanelProps) {
  const workspaceId = node.workspace.id;
  const selection = useWorktreeRemovalSelection(workspaceId, node.worktrees, onRemoveWorktrees);
  const hasListRows = node.worktrees.length > 0;
  const selectEntry = (entry: WorktreeNestEntry) => {
    if (!entry.selectable) return;
    onSelectWorktree?.(entry);
    onClose?.();
  };

  const rows = node.worktrees.map(entry => {
    if (selection.mode) {
      const reason = selection.reason(entry);
      const selected = selection.selectedIds.has(entry.key);
      const content = (
        <WorktreeNestRow
          entry={{ ...entry, adoptable: false }}
          userHomeDir={userHomeDir}
          selection={{ selected, reason }}
          className="w-full"
        />
      );
      return variant === "menu" ? (
        <MenubarCheckboxItem
          key={entry.key}
          checked={selected}
          // The leading selection mark replaces the item's trailing indicator,
          // which would otherwise read as the scope check.
          className="group/wtnest min-w-0 pr-2.5 *:data-[slot=dropdown-menu-checkbox-item-indicator]:hidden"
          disabled={Boolean(reason)}
          closeOnClick={false}
          aria-label={`Select ${entry.name}`}
          title={reason ?? undefined}
          onClick={event => selection.toggle(entry, event.shiftKey)}
        >
          {content}
        </MenubarCheckboxItem>
      ) : (
        <button
          key={entry.key}
          type="button"
          role="checkbox"
          aria-checked={selected}
          disabled={Boolean(reason)}
          aria-label={`Select ${entry.name}`}
          title={reason ?? undefined}
          className="group/wtnest flex min-h-control-compact w-full min-w-0 items-center rounded-md px-1.5 py-1 text-left hover:bg-surface-2 disabled:hover:bg-transparent"
          onClick={event => selection.toggle(entry, event.shiftKey)}
        >
          {content}
        </button>
      );
    }
    const checked = entry.worktree != null && entry.worktree.id === selectedWorktreeId;
    const actions = (
      <WorktreeSubmenuRowActions
        entry={entry}
        cleanupReason={selection.reason(entry)}
        testIdPrefix={testIdPrefix}
        variant={variant}
        onRemoveWorktree={onRemoveWorktree}
        onResolveMissing={onResolveMissing}
        onOpenContext={onOpenContext}
      />
    );
    const body = <WorktreeNestRow entry={entry} userHomeDir={userHomeDir} checked={checked} />;
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
            "hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-none focus-visible:shadow-focus-ring",
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
        {hasListRows && !selection.enabled ? <MenubarSeparator /> : null}
        <MenubarItem
          className={cn(FOOTER_ROW_CLASS, "text-muted")}
          data-testid={`${testIdPrefix}-worktree-create-${workspaceId}`}
          onClick={() => {
            onCreateWorktree();
            onClose?.();
          }}
        >
          <Plus className="size-3.5 text-faint" />
          New worktree
        </MenubarItem>
      </>
    ) : (
      <>
        {hasListRows && !selection.enabled ? <div className="my-1 h-px bg-line-soft" /> : null}
        <button
          type="button"
          data-testid={`${testIdPrefix}-worktree-create-${workspaceId}`}
          className={cn(
            FOOTER_ROW_CLASS,
            "flex w-full items-center gap-2 rounded-md px-2 text-small-body text-muted hover:bg-surface-2"
          )}
          onClick={() => {
            onCreateWorktree();
            onClose?.();
          }}
        >
          <Plus className="size-3.5 text-faint" />
          New worktree
        </button>
      </>
    )
  ) : null;

  if (variant === "menu") {
    return (
      <WorktreeNest
        onKeyDown={selection.onKeyDown}
        viewportClassName={MENU_VIEWPORT_MAX_CLASS}
        footer={
          <>
            <WorktreeSelectionToolbar selection={selection} menu={variant === "menu"} />
            {create}
          </>
        }
      >
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
      onKeyDown={selection.onKeyDown}
      role="group"
      aria-label={`Worktrees in ${node.workspace.name}`}
      className={PANEL_FRAME_MAX_CLASS}
      viewportClassName={PANEL_VIEWPORT_MAX_CLASS}
      listClassName="pr-2"
      footer={
        <>
          <WorktreeSelectionToolbar selection={selection} menu={false} />
          {create}
        </>
      }
    >
      {rows}
    </WorktreeNest>
  );
}
