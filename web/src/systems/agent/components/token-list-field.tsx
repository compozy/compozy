import { Plus } from "lucide-react";
import { useId, useState, type KeyboardEvent } from "react";

import {
  CommandSelectChip,
  CommandSelectChipStrip,
  Field,
  FieldDescription,
  FieldError,
  FieldHeader,
  FieldLabel,
  HelpTip,
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@compozy/ui";

import { appendAgentCreateTokens, removeAgentCreateToken } from "../lib/agent-create-draft";

export interface TokenListFieldProps {
  /** Visible caption under the label. Prefer `help` for explanatory prose. */
  description?: string;
  disabled?: boolean;
  readOnly?: boolean;
  error?: string;
  /** Explanatory prose, shown as a `HelpTip` beside the label. */
  help?: string;
  label: string;
  onChange: (values: string[]) => void;
  placeholder: string;
  testId: string;
  values: string[];
}

export function TokenListField({
  description,
  disabled = false,
  readOnly = false,
  error,
  help,
  label,
  onChange,
  placeholder,
  testId,
  values,
}: TokenListFieldProps) {
  const inputId = useId();
  const [inputValue, setInputValue] = useState("");

  const commit = () => {
    if (disabled || readOnly || inputValue.trim().length === 0) return;
    onChange(appendAgentCreateTokens(values, inputValue));
    setInputValue("");
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === ",") {
      event.preventDefault();
      commit();
    }
  };

  return (
    <Field data-invalid={Boolean(error)}>
      {help ? (
        <FieldHeader>
          <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
          <HelpTip label={`About ${label.toLowerCase()}`}>{help}</HelpTip>
        </FieldHeader>
      ) : (
        <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
      )}
      {description ? <FieldDescription>{description}</FieldDescription> : null}
      <InputGroup>
        <InputGroupInput
          aria-disabled={readOnly || undefined}
          aria-invalid={Boolean(error)}
          data-testid={testId + "-input"}
          disabled={disabled}
          readOnly={readOnly}
          id={inputId}
          onBlur={commit}
          onChange={event => {
            if (disabled || readOnly) return;
            const next = event.target.value;
            if (/[,\n]/.test(next)) {
              onChange(appendAgentCreateTokens(values, next));
              setInputValue("");
              return;
            }
            setInputValue(next);
          }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          value={inputValue}
        />
        <InputGroupAddon align="inline-end">
          <InputGroupButton
            aria-label={"Add " + label.toLowerCase()}
            data-testid={testId + "-add"}
            disabled={disabled || inputValue.trim().length === 0}
            aria-disabled={readOnly || undefined}
            onClick={commit}
            size="icon-xs"
          >
            <Plus aria-hidden="true" />
          </InputGroupButton>
        </InputGroupAddon>
      </InputGroup>
      {values.length > 0 ? (
        <CommandSelectChipStrip data-testid={testId + "-tokens"}>
          {values.map(value => (
            <CommandSelectChip
              aria-disabled={readOnly || undefined}
              aria-label={readOnly ? undefined : "Remove " + value}
              disabled={disabled}
              key={value}
              onRemove={
                readOnly ? undefined : () => onChange(removeAgentCreateToken(values, value))
              }
            >
              {value}
            </CommandSelectChip>
          ))}
        </CommandSelectChipStrip>
      ) : null}
      <FieldError data-testid={testId + "-error"}>{error}</FieldError>
    </Field>
  );
}
