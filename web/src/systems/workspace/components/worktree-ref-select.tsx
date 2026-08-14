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
  disabled = false,
  emptyOption,
  footer,
  placeholder = "Select a worktree",
  testId,
  triggerId,
  displayValue,
  invalid = false,
  describedBy,
  className,
}: WorktreeRefSelectProps) {
  const [open, setOpen] = useState(false);
  const ready = worktrees.filter(worktree => worktree.state === "ready");
  const selected = worktrees.find(worktree => worktree.id === value);
  const isEmptySelection = emptyOption !== undefined && value === emptyOption.value;
  const missing = value !== "" && !isEmptySelection && selected === undefined;

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
        aria-invalid={invalid || missing || undefined}
        aria-describedby={describedBy}
        className={cn("w-full justify-between gap-2", className)}
        data-missing={missing ? "" : undefined}
        data-testid={testId}
        disabled={disabled}
        selected={Boolean(selected) || isEmptySelection || Boolean(displayValue)}
      >
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          {missing ? <WorktreeStateChip state="missing" /> : null}
          <span className="min-w-0 flex-1 truncate text-small-body text-fg">
            {displayValue ??
              (isEmptySelection ? emptyOption.label : (selected?.name ?? (value || placeholder)))}
          </span>
        </span>
        <ChevronsUpDown aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      </CommandSelectTrigger>
      <CommandSelectShell className="min-w-64" inputPlaceholder="Search worktrees...">
        <CommandList>
          <CommandEmpty>No worktrees match your search.</CommandEmpty>
          {emptyOption ? (
            <CommandSelectGroup>
              <CommandItem
                onSelect={() => handleSelect(emptyOption.value)}
                value={emptyOption.label}
              >
                {emptyOption.label}
              </CommandItem>
            </CommandSelectGroup>
          ) : null}
          {ready.length > 0 ? (
            <CommandSelectGroup heading="Worktrees">
              {ready.map(worktree => (
                <CommandItem
                  key={worktree.id}
                  onSelect={() => handleSelect(worktree.id)}
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
      </CommandSelectShell>
    </CommandSelect>
  );
}
