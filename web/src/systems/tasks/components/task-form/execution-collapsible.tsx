import { useState } from "react";
import { ChevronRight } from "lucide-react";

import {
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Field,
  FieldContent,
  FieldDescription,
  FieldTitle,
  Switch,
} from "@agh/ui";

interface ExecutionCollapsibleProps {
  saveAsDraft: boolean;
  autoEnqueueOnReady: boolean;
  onSaveAsDraft: (value: boolean) => void;
  onAutoEnqueue: (value: boolean) => void;
}

/**
 * Execution — collapsible draft / auto-enqueue switches. The trailing badge
 * mirrors the draft state (`Saved as draft` vs `Enqueue on create`) so the
 * effect is visible without expanding.
 */
export function ExecutionCollapsible({
  saveAsDraft,
  autoEnqueueOnReady,
  onSaveAsDraft,
  onAutoEnqueue,
}: ExecutionCollapsibleProps) {
  const [open, setOpen] = useState(false);
  const badge = saveAsDraft ? "Saved as draft" : "Enqueue on create";

  return (
    <Collapsible className="mt-5 border-t border-line-soft pt-1" onOpenChange={setOpen} open={open}>
      <CollapsibleTrigger
        className="flex w-full items-center gap-2 rounded-sm py-2.5 text-left outline-none focus-visible:shadow-focus-ring"
        data-testid="task-execution-toggle"
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn("size-4 text-muted transition-transform", open && "rotate-90")}
        />
        <span className="flex-1 text-small-body font-semibold text-fg-strong">Execution</span>
        <span className="font-mono text-form-hint text-subtle">{badge}</span>
      </CollapsibleTrigger>

      <CollapsibleContent className="flex flex-col gap-4 pt-2 pb-1">
        <Field orientation="horizontal">
          <Switch
            checked={saveAsDraft}
            data-testid="task-save-draft-toggle"
            onCheckedChange={onSaveAsDraft}
          />
          <FieldContent>
            <FieldTitle>Save as draft</FieldTitle>
            <FieldDescription>
              Create the contract without enqueueing a run. Enqueue it later from the task.
            </FieldDescription>
          </FieldContent>
        </Field>

        <Field orientation="horizontal">
          <Switch
            checked={autoEnqueueOnReady}
            data-testid="task-auto-enqueue-toggle"
            onCheckedChange={onAutoEnqueue}
          />
          <FieldContent>
            <FieldTitle>Auto-enqueue when ready</FieldTitle>
            <FieldDescription>
              Once dependencies resolve, queue a run automatically without manual action.
            </FieldDescription>
          </FieldContent>
        </Field>
      </CollapsibleContent>
    </Collapsible>
  );
}
