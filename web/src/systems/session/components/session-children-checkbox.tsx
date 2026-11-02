import { Button, Checkbox, Field, FieldLabel } from "@compozy/ui";

import {
  childSessionCountLabel,
  type SessionChildrenChoice,
  type SessionTreeAction,
} from "../lib/session-tree";

export interface SessionChildrenCheckboxProps {
  action: SessionTreeAction;
  choice: SessionChildrenChoice;
  disabled?: boolean;
}

/** The include-children answer shared by the archive and delete confirmations. */
export function SessionChildrenCheckbox({
  action,
  choice,
  disabled = false,
}: SessionChildrenCheckboxProps) {
  const id = `session-children-${action}`;
  const retry = choice.onRetryRead;
  return (
    <>
      <Field orientation="horizontal">
        <Checkbox
          id={id}
          checked={choice.included}
          disabled={disabled || choice.pending}
          data-testid="session-children-include"
          onCheckedChange={checked => choice.onIncludedChange(checked === true)}
        />
        <FieldLabel htmlFor={id}>
          Also {action} {choice.pending ? "child sessions" : childSessionCountLabel(choice.count)}
        </FieldLabel>
      </Field>
      {retry ? (
        <p role="alert" className="flex items-center gap-2 text-small-body text-destructive">
          Couldn't read child sessions.
          <Button type="button" variant="ghost" size="sm" onClick={retry}>
            Try again
          </Button>
        </p>
      ) : null}
    </>
  );
}
