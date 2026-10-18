import { useId } from "react";

import {
  Disclosure,
  Field,
  FieldContent,
  FieldHeader,
  FieldTitle,
  HelpTip,
  Switch,
} from "@compozy/ui";

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
  const badge = saveAsDraft ? "Saved as draft" : "Starts on create";
  const draftLabelId = useId();
  const autoEnqueueLabelId = useId();

  return (
    <Disclosure
      className="mt-5 border-t border-line-soft pt-1"
      label={
        <>
          <span className="flex-1 font-semibold text-fg-strong">Execution</span>
          <span className="text-form-hint font-normal text-subtle">{badge}</span>
        </>
      }
      size="md"
      triggerProps={{ "data-testid": "task-execution-toggle", className: "w-full py-2.5" }}
      contentProps={{ className: "flex flex-col gap-4 pb-1" }}
    >
      <Field orientation="horizontal">
        <Switch
          aria-labelledby={draftLabelId}
          checked={saveAsDraft}
          data-testid="task-save-draft-toggle"
          onCheckedChange={onSaveAsDraft}
        />
        <FieldContent>
          <FieldHeader>
            <FieldTitle id={draftLabelId}>Save as draft</FieldTitle>
            <HelpTip label="About save as draft">
              Create the task without starting it. Start it later from the task page.
            </HelpTip>
          </FieldHeader>
        </FieldContent>
      </Field>

      <Field orientation="horizontal">
        <Switch
          aria-labelledby={autoEnqueueLabelId}
          checked={autoEnqueueOnReady}
          data-testid="task-auto-enqueue-toggle"
          onCheckedChange={onAutoEnqueue}
        />
        <FieldContent>
          <FieldHeader>
            <FieldTitle id={autoEnqueueLabelId}>Start automatically when ready</FieldTitle>
            <HelpTip label="About starting automatically">
              Once the tasks it waits on finish, start it without you doing anything.
            </HelpTip>
          </FieldHeader>
        </FieldContent>
      </Field>
    </Disclosure>
  );
}
