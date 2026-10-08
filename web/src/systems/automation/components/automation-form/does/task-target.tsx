import {
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Textarea,
} from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import type { AutomationFormDraft } from "../../../lib/automation-form-draft";

type TaskDraft = NonNullable<AutomationFormDraft["task"]>;
type OwnerKind = NonNullable<TaskDraft["owner"]>["kind"];

const OWNER_CHOICES: ReadonlyArray<{ value: OwnerKind | ""; label: string }> = [
  { value: "pool", label: "Agent pool" },
  { value: "human", label: "Person" },
  { value: "", label: "Anyone available" },
];

/** Owner kinds the daemon accepts that the simple picker doesn't offer; kept when editing. */
const OTHER_OWNER_LABELS: Partial<Record<OwnerKind, string>> = {
  agent_session: "Agent session",
  automation: "Automation",
  extension: "Extension",
};

interface TaskTargetProps {
  disabled: boolean;
  form: AutomationFormModel;
  name: string;
  task: TaskDraft;
}

/** Create a task (schedules only): each run creates a new task for a pool, a person or anyone. */
export function TaskTarget({ disabled, form, name, task }: TaskTargetProps) {
  const ownerKind = task.owner?.kind ?? "";
  const otherLabel = ownerKind === "" ? undefined : OTHER_OWNER_LABELS[ownerKind];

  return (
    <div className="flex flex-col gap-4">
      <Field>
        <FieldLabel htmlFor="automation-task-title">Task title</FieldLabel>
        <Input
          data-testid="automation-task-title"
          disabled={disabled}
          id="automation-task-title"
          onChange={event => form.onTaskTitle(event.target.value)}
          placeholder={name}
          value={task.title ?? ""}
        />
      </Field>
      <div className="grid grid-cols-2 gap-3">
        <Field>
          <FieldLabel htmlFor="automation-task-owner-kind">For</FieldLabel>
          <NativeSelect
            className="w-full"
            data-testid="automation-task-owner-kind"
            disabled={disabled}
            id="automation-task-owner-kind"
            onChange={event => form.onOwnerKind(event.target.value as OwnerKind | "")}
            value={ownerKind}
          >
            {OWNER_CHOICES.map(choice => (
              <NativeSelectOption key={choice.label} value={choice.value}>
                {choice.label}
              </NativeSelectOption>
            ))}
            {otherLabel ? (
              <NativeSelectOption value={ownerKind}>{otherLabel}</NativeSelectOption>
            ) : null}
          </NativeSelect>
        </Field>
        <Field>
          <FieldLabel htmlFor="automation-task-owner-ref">Name</FieldLabel>
          <Input
            className="font-mono text-form-label"
            data-testid="automation-task-owner-ref"
            disabled={disabled || ownerKind === ""}
            id="automation-task-owner-ref"
            onChange={event => form.onOwnerRef(event.target.value)}
            placeholder={ownerKind === "human" ? "alex" : "reviewers"}
            value={task.owner?.ref ?? ""}
          />
        </Field>
      </div>
      <Field>
        <FieldLabel htmlFor="automation-task-description">Description</FieldLabel>
        <Textarea
          data-testid="automation-task-description"
          disabled={disabled}
          id="automation-task-description"
          onChange={event => form.onTaskDescription(event.target.value)}
          value={task.description ?? ""}
        />
        <FieldDescription>
          Each run creates a new task. The task handles its own retries.
        </FieldDescription>
      </Field>
    </div>
  );
}
