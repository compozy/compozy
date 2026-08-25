import { useState } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
  Spinner,
} from "@compozy/ui";

export interface SkillExposeTarget {
  slug: string;
  label: string;
  /** The folder convention this target writes into, e.g. `.agents`. */
  hint: string | null;
}

interface SkillExposeTargetPickerProps {
  targets: readonly SkillExposeTarget[];
  disabled: boolean;
  loading: boolean;
  error: string | null;
  onExpose: (targets: string[]) => void;
  onRetry?: () => void;
}

const TEST_ID = "skill-expose-target-picker";

/**
 * Picks the sources to link this skill into.
 *
 * The list holds only enabled presets: a source that is turned off is absent
 * rather than greyed out, because offering it would be offering a dead end.
 * Compozy never appears — it is the skill's home, not a target.
 */
export function SkillExposeTargetPicker({
  targets,
  disabled,
  loading,
  error,
  onExpose,
  onRetry,
}: SkillExposeTargetPickerProps) {
  const [open, setOpen] = useState(false);
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set());

  const toggle = (slug: string) => {
    setSelected(current => {
      const next = new Set(current);
      if (!next.delete(slug)) next.add(slug);
      return next;
    });
  };

  const commit = () => {
    if (selected.size === 0) return;
    onExpose([...selected]);
    setSelected(new Set());
    setOpen(false);
  };

  if (loading) {
    return (
      <p
        className="flex items-center gap-2 text-form-hint text-subtle"
        data-testid={`${TEST_ID}-loading`}
        role="status"
      >
        <Spinner aria-hidden="true" className="size-3.5" />
        Loading sources…
      </p>
    );
  }

  if (error !== null) {
    return (
      <div className="flex flex-wrap items-center gap-2" data-testid={`${TEST_ID}-error`}>
        <p className="text-form-hint text-danger" role="alert">
          {error}
        </p>
        {onRetry ? (
          <Button onClick={onRetry} size="sm" type="button" variant="ghost">
            Retry
          </Button>
        ) : null}
      </div>
    );
  }

  if (targets.length === 0) {
    return (
      <p className="text-form-hint text-subtle" data-testid={`${TEST_ID}-none`}>
        No source is turned on to expose into. Turn one on in Settings &rsaquo; Skills.
      </p>
    );
  }

  return (
    <DropdownMenu
      onOpenChange={next => {
        setOpen(next);
        if (!next) setSelected(new Set());
      }}
      open={open}
    >
      <DropdownMenuTrigger
        render={
          <Button
            data-testid={`${TEST_ID}-trigger`}
            disabled={disabled}
            size="sm"
            variant="outline"
          >
            Expose to…
          </Button>
        }
      />
      <DropdownMenuContent align="start" className="min-w-56" data-testid={`${TEST_ID}-menu`}>
        <DropdownMenuLabel>Expose to</DropdownMenuLabel>
        {targets.map(target => (
          <DropdownMenuCheckboxItem
            checked={selected.has(target.slug)}
            className="gap-2"
            data-testid={`${TEST_ID}-option-${target.slug}`}
            key={target.slug}
            onCheckedChange={() => toggle(target.slug)}
            onSelect={event => event.preventDefault()}
          >
            <span className="min-w-0 flex-1 truncate text-fg">{target.label}</span>
            {target.hint !== null ? (
              <span className="shrink-0 font-mono text-badge text-faint">{target.hint}</span>
            ) : null}
          </DropdownMenuCheckboxItem>
        ))}
        <div className="mt-1 flex items-center justify-end gap-1.5 border-t border-line pt-1.5">
          <Button onClick={() => setOpen(false)} size="sm" type="button" variant="ghost">
            Cancel
          </Button>
          <Button
            data-testid={`${TEST_ID}-confirm`}
            disabled={selected.size === 0}
            onClick={commit}
            size="sm"
            type="button"
          >
            Expose
          </Button>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
