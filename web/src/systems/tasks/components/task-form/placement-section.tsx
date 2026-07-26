import { Field, FieldDescription, FieldLabel, Input } from "@agh/ui";

interface PlacementSectionProps {
  parentTaskId: string;
  onParentTaskId: (value: string) => void;
}

/**
 * Placement — where the task sits in the hierarchy. A parent identifier makes
 * this a child in an epic; scope and workspace are set elsewhere in the form.
 */
export function PlacementSection({ parentTaskId, onParentTaskId }: PlacementSectionProps) {
  return (
    <Field>
      <FieldLabel htmlFor="task-parent-input">
        Parent task
        <span className="font-normal text-faint"> (makes this a child in an epic)</span>
      </FieldLabel>
      <Input
        className="font-mono"
        data-testid="task-parent-input"
        id="task-parent-input"
        onChange={event => onParentTaskId(event.target.value)}
        placeholder="Search by identifier or task id (e.g. TASK-118)"
        value={parentTaskId}
      />
      <FieldDescription>
        A parent makes this a child task in an epic. Global tasks aren&apos;t bound to a workspace;
        any owner across the runtime can claim them.
      </FieldDescription>
    </Field>
  );
}
