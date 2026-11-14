import {
  Checkbox,
  Field,
  FieldContent,
  FieldDescription,
  FieldLabel,
  Input,
  TreeBranch,
  TreeBranchNode,
} from "@compozy/ui";

import type { CleanupChoiceCounts, CleanupWhat } from "../../lib/archived-cleanup-model";

export function CountBadge({ count }: { count: number }) {
  return (
    <span className="text-meta text-muted-foreground tabular-nums">{count.toLocaleString()}</span>
  );
}

/** One check with its plain consequence and the sessions it alone would remove. */
function CheckRow({
  id,
  label,
  description,
  count,
  checked,
  disabled,
  onCheckedChange,
}: {
  id: string;
  label: string;
  description: string;
  count: number;
  checked: boolean;
  disabled: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <Field className="py-1" orientation="horizontal">
      <Checkbox
        aria-describedby={`${id}-hint`}
        checked={checked}
        data-testid={id}
        disabled={disabled}
        id={id}
        onCheckedChange={next => onCheckedChange(next === true)}
      />
      <FieldContent className="min-w-0">
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        <FieldDescription id={`${id}-hint`}>{description}</FieldDescription>
      </FieldContent>
      <CountBadge count={count} />
    </Field>
  );
}

/**
 * Everything on top, its two parts hanging from it on the branch the session list draws for child
 * sessions: checking Everything checks and locks them, because deleting a session takes all it
 * holds. Unchecked, the parts thin sessions out instead.
 */
export function WhatChecks({
  what,
  counts,
  disabled,
  onChange,
}: {
  what: CleanupWhat;
  counts: CleanupChoiceCounts;
  disabled: boolean;
  onChange: (next: Partial<CleanupWhat>) => void;
}) {
  const included = "Included in Everything.";
  return (
    <div
      aria-labelledby="archived-cleanup-what-label"
      className="flex flex-col rounded-lg bg-card px-3 py-1.5 shadow-card"
      data-testid="archived-cleanup-what"
      role="group"
    >
      <CheckRow
        checked={what.everything}
        count={counts.everything}
        description="Archived sessions, with their child sessions and subagents."
        disabled={disabled}
        id="archived-cleanup-what-everything"
        label="Everything"
        onCheckedChange={everything => onChange({ everything })}
      />
      {/* The rail continues from the center of Everything's check; each arm lands on a part's check. */}
      <TreeBranch className="-ml-1.25">
        <TreeBranchNode className="pl-1.5 [--branch-arm:0.8125rem]">
          <CheckRow
            checked={what.everything || what.children}
            count={counts.children}
            description={what.everything ? included : "The main sessions stay."}
            disabled={disabled || what.everything}
            id="archived-cleanup-what-children"
            label="Child sessions"
            onCheckedChange={children => onChange({ children })}
          />
        </TreeBranchNode>
        <TreeBranchNode className="pl-1.5 [--branch-arm:0.8125rem]">
          <CheckRow
            checked={what.everything || what.subagents}
            count={counts.subagents}
            description={what.everything ? included : "Main and child sessions stay."}
            disabled={disabled || what.everything}
            id="archived-cleanup-what-subagents"
            label="Subagents"
            onCheckedChange={subagents => onChange({ subagents })}
          />
        </TreeBranchNode>
      </TreeBranch>
    </div>
  );
}

/** Above the threshold the person types the count; separators are optional. */
export function TypedConfirm({
  total,
  value,
  onChange,
}: {
  total: number;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <Field>
      <FieldLabel htmlFor="archived-cleanup-typed">
        <span>
          Type <span className="tabular-nums">{total.toLocaleString()}</span> to confirm
        </span>
      </FieldLabel>
      <Input
        autoComplete="off"
        data-testid="archived-cleanup-typed"
        id="archived-cleanup-typed"
        inputMode="numeric"
        onChange={event => onChange(event.target.value)}
        placeholder={total.toLocaleString()}
        value={value}
      />
    </Field>
  );
}
