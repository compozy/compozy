import { ChevronsUpDown } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  CommandEmpty,
  CommandItem,
  CommandList,
  CommandSelect,
  CommandSelectGroup,
  CommandSelectShell,
  CommandSelectTrigger,
  MonoId,
  cn,
} from "@compozy/ui";

import type { WorktreePayload } from "../types";
import { WorktreeStateChip } from "./worktree-state-chip";

export interface WorktreeRefSelectProps {
  /** The selected worktree id, or the empty-option value. */
  value: string;
  worktrees: readonly WorktreePayload[];
  onChange: (next: string) => void;
  ariaLabel: string;
  disabled?: boolean;
  /** A leading choice that means "no worktree" (e.g. Workspace root). */
  emptyOption?: { value: string; label: string };
  /** Trailing action row, e.g. "New worktree…". */
  footer?: ReactNode;
  placeholder?: string;
  testId?: string;
  triggerId?: string;
  displayValue?: string;
  invalid?: boolean;
  describedBy?: string;
  className?: string;
}

interface WorktreeRefSelection {
  ready: readonly WorktreePayload[];
  selected: WorktreePayload | undefined;
  isEmptySelection: boolean;
  missing: boolean;
}

function resolveWorktreeRefSelection(
  value: string,
  worktrees: readonly WorktreePayload[],
  emptyOption: WorktreeRefSelectProps["emptyOption"]
): WorktreeRefSelection {
  const selected = worktrees.find(worktree => worktree.id === value);
  const isEmptySelection = emptyOption !== undefined && value === emptyOption.value;
  return {
    ready: worktrees.filter(worktree => worktree.state === "ready"),
    selected,
    isEmptySelection,
    missing: value !== "" && !isEmptySelection && selected === undefined,
  };
}

/** Trigger text: an explicit display value wins, then the empty choice, the row, the raw id. */
function worktreeRefTriggerText(
  value: string,
  selection: WorktreeRefSelection,
  emptyOption: WorktreeRefSelectProps["emptyOption"],
  placeholder: string,
  displayValue: string | undefined
): string {
  if (displayValue !== undefined) return displayValue;
  if (selection.isEmptySelection && emptyOption) return emptyOption.label;
  return selection.selected?.name ?? (value || placeholder);
}

function WorktreeRefOptions({
  ready,
  emptyOption,
  footer,
  onSelect,
}: {
  ready: readonly WorktreePayload[];
  emptyOption: WorktreeRefSelectProps["emptyOption"];
  footer: ReactNode;
  onSelect: (next: string) => void;
}) {
  return (
    <CommandList>
      <CommandEmpty>No worktrees match your search.</CommandEmpty>
      {emptyOption ? (
        <CommandSelectGroup>
          <CommandItem onSelect={() => onSelect(emptyOption.value)} value={emptyOption.label}>
            {emptyOption.label}
          </CommandItem>
        </CommandSelectGroup>
      ) : null}
      {ready.length > 0 ? (
        <CommandSelectGroup heading="Worktrees">
          {ready.map(worktree => (
            <CommandItem
              key={worktree.id}
              onSelect={() => onSelect(worktree.id)}
              value={worktree.name}
            >
              <span className="min-w-0 flex-1 truncate text-small-body text-fg">
                {worktree.name}
              </span>
              <MonoId size="sm" value={worktree.branch} />
            </CommandItem>
          ))}
        </CommandSelectGroup>
      ) : null}
      {footer ? <CommandSelectGroup>{footer}</CommandSelectGroup> : null}
    </CommandList>
  );
}

/**
 * Picks one worktree inside a single workspace.
 *
 * Only `ready` rows are offered: a pending worktree has no usable checkout and a
 * missing one has no directory at all, so listing either would be an offer the
 * runtime cannot honour. A value that no longer resolves is shown with the
 * missing chip instead of silently falling back — the caller decides what to do
 * about it.
 */
export function WorktreeRefSelect({
  value,
  worktrees,
  onChange,
  ariaLabel,
  disabled,
  emptyOption,
  footer,
  placeholder = "Select a worktree",
  testId,
  triggerId,
  displayValue,
  invalid,
  describedBy,
  className,
}: WorktreeRefSelectProps) {
  const [open, setOpen] = useState(false);
  const selection = resolveWorktreeRefSelection(value, worktrees, emptyOption);
  const { missing } = selection;
  const hasSelection =
    selection.selected !== undefined || selection.isEmptySelection || Boolean(displayValue);

  function handleSelect(next: string) {
    onChange(next);
    setOpen(false);
  }

  return (
    <CommandSelect onOpenChange={setOpen} open={open}>
      <CommandSelectTrigger
        id={triggerId}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={ariaLabel}
        aria-invalid={invalid === true || missing || undefined}
        aria-describedby={describedBy}
        className={cn("w-full justify-between gap-2", className)}
        data-missing={missing ? "" : undefined}
        data-testid={testId}
        disabled={disabled === true}
        selected={hasSelection}
      >
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          {missing ? <WorktreeStateChip state="missing" /> : null}
          <span className="min-w-0 flex-1 truncate text-small-body text-fg">
            {worktreeRefTriggerText(value, selection, emptyOption, placeholder, displayValue)}
          </span>
        </span>
        <ChevronsUpDown aria-hidden="true" className="size-4 shrink-0 text-subtle" />
      </CommandSelectTrigger>
      <CommandSelectShell className="min-w-64" inputPlaceholder="Search worktrees...">
        <WorktreeRefOptions
          emptyOption={emptyOption}
          footer={footer}
          onSelect={handleSelect}
          ready={selection.ready}
        />
      </CommandSelectShell>
    </CommandSelect>
  );
}
