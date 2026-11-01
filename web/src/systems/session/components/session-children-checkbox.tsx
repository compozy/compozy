import { Checkbox, Field, FieldLabel } from "@compozy/ui";

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
  return (
    <Field orientation="horizontal">
      <Checkbox
        id={id}
        checked={choice.included}
        disabled={disabled}
        data-testid="session-children-include"
        onCheckedChange={checked => choice.onIncludedChange(checked === true)}
      />
      <FieldLabel htmlFor={id}>
        Also {action} {childSessionCountLabel(choice.count)}
      </FieldLabel>
    </Field>
  );
}
