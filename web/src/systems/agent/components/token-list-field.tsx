import { Plus, X } from "lucide-react";
import { useId, useState, type KeyboardEvent } from "react";

import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  Pill,
} from "@agh/ui";

import { appendAgentCreateTokens, removeAgentCreateToken } from "../lib/agent-create-draft";

export interface TokenListFieldProps {
  description: string;
  disabled?: boolean;
  readOnly?: boolean;
  error?: string;
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
      <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
      <FieldDescription>{description}</FieldDescription>
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
            <Plus aria-hidden="true" className="size-3" />
          </InputGroupButton>
        </InputGroupAddon>
      </InputGroup>
      {values.length > 0 ? (
        <div className="flex flex-wrap gap-1.5" data-testid={testId + "-tokens"}>
          {values.map(value => (
            <Pill key={value} className="gap-1 pr-1" size="sm">
              <span className="max-w-44 truncate">{value}</span>
              <button
                aria-label={"Remove " + value}
                className="inline-flex size-4 items-center justify-center rounded-sm text-subtle transition-colors hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring disabled:pointer-events-none disabled:opacity-50"
                disabled={disabled}
                aria-disabled={readOnly || undefined}
                onClick={() => {
                  if (disabled || readOnly) return;
                  onChange(removeAgentCreateToken(values, value));
                }}
                type="button"
              >
                <X aria-hidden="true" className="size-3" />
              </button>
            </Pill>
          ))}
        </div>
      ) : null}
      <FieldError data-testid={testId + "-error"}>{error}</FieldError>
    </Field>
  );
}
