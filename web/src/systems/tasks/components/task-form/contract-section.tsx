import { Eyebrow, Field, FieldDescription, FieldLabel, Input, Textarea } from "@agh/ui";

interface ContractSectionProps {
  title: string;
  description: string;
  onTitle: (value: string) => void;
  onDescription: (value: string) => void;
}

/**
 * The durable contract inputs — task title (required) and an outcome
 * description. The title label row trails a `Required` eyebrow to mark the
 * single mandatory field in the form.
 */
export function ContractSection({
  title,
  description,
  onTitle,
  onDescription,
}: ContractSectionProps) {
  return (
    <div className="flex flex-col gap-4">
      <Field>
        <div className="flex items-center justify-between gap-3">
          <FieldLabel htmlFor="task-title-input">Title</FieldLabel>
          <Eyebrow className="text-subtle">Required</Eyebrow>
        </div>
        <Input
          data-testid="task-title-input"
          id="task-title-input"
          onChange={event => onTitle(event.target.value)}
          placeholder="Generate API client for payments-v3"
          required
          value={title}
        />
      </Field>

      <Field>
        <FieldLabel htmlFor="task-description-input">Description</FieldLabel>
        <FieldDescription>
          Describe the expected outcome, constraints, and completion criteria.
        </FieldDescription>
        <Textarea
          className="min-h-form-textarea"
          data-testid="task-description-input"
          id="task-description-input"
          onChange={event => onDescription(event.target.value)}
          placeholder="Describe the task contract for the agent."
          value={description}
        />
      </Field>
    </div>
  );
}
