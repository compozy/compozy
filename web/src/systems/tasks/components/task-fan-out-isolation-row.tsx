import { Checkbox, Field, Label } from "@compozy/ui";

interface TaskFanOutIsolationRowProps {
  checked: boolean;
  /** Non-empty brief lines — the same count the request will carry. */
  designationCount: number;
  disabled?: boolean;
  onCheckedChange: (checked: boolean) => void;
}

const COUNT_DESCRIPTION_ID = "tasks-fan-out-isolation-count";

/**
 * Opt-in per-run isolation.
 *
 * The consequence is stated from the same parsed brief lines the payload uses,
 * so the number on screen can never disagree with the number of worktrees the
 * request creates. With no briefs typed there is nothing to promise, so the
 * statement is absent rather than zero.
 */
export function TaskFanOutIsolationRow({
  checked,
  designationCount,
  disabled = false,
  onCheckedChange,
}: TaskFanOutIsolationRowProps) {
  const showCount = checked && designationCount > 0;

  return (
    <Field data-on={checked ? "" : undefined} data-slot="task-fan-out-isolation">
      <Label className="flex cursor-pointer items-start gap-2.5 rounded-md border border-line-soft bg-canvas-tint px-3 py-[11px]">
        <Checkbox
          aria-describedby={showCount ? COUNT_DESCRIPTION_ID : undefined}
          checked={checked}
          data-testid="tasks-fan-out-worktree-per-run"
          disabled={disabled}
          onCheckedChange={onCheckedChange}
        />
        <span>
          <span
            className="block text-small-body font-medium text-fg"
            data-slot="task-fan-out-isolation-title"
          >
            Isolate each run in its own worktree
          </span>
          {showCount ? (
            <span
              className="mt-[3px] block text-badge leading-[1.45] text-muted"
              data-slot="task-fan-out-isolation-count"
              id={COUNT_DESCRIPTION_ID}
            >
              {designationCount === 1
                ? "Creates 1 worktree, one per run."
                : `Creates ${designationCount} worktrees, one per run.`}
            </span>
          ) : null}
        </span>
      </Label>
    </Field>
  );
}
