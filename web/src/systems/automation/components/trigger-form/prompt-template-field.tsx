import { useRef } from "react";

import { Field, FieldDescription, FieldLabel, Textarea } from "@agh/ui";

import { VariableBar } from "./variable-bar";

interface PromptTemplateFieldProps {
  value: string;
  variables: string[];
  onChange: (next: string) => void;
}

/** Mono prompt textarea with a click-to-insert variable bar. */
export function PromptTemplateField({ value, variables, onChange }: PromptTemplateFieldProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const insertVariable = (variable: string) => {
    const textarea = textareaRef.current;
    const start = textarea?.selectionStart ?? value.length;
    const end = textarea?.selectionEnd ?? value.length;
    const next = `${value.slice(0, start)}${variable}${value.slice(end)}`;
    onChange(next);

    const caret = start + variable.length;
    // Restore the caret after React commits the controlled value.
    requestAnimationFrame(() => {
      if (!textarea) return;
      textarea.focus();
      textarea.setSelectionRange(caret, caret);
    });
  };

  return (
    <Field>
      <FieldLabel htmlFor="trigger-prompt">
        Prompt template
        <span className="font-normal text-faint"> (click a variable to insert it)</span>
      </FieldLabel>
      <VariableBar onInsert={insertVariable} variables={variables} />
      <Textarea
        className="min-h-28"
        data-testid="trigger-prompt-input"
        id="trigger-prompt"
        onChange={event => onChange(event.target.value)}
        ref={textareaRef}
        value={value}
        variant="mono"
      />
      <FieldDescription>
        Go <code className="font-mono text-mono-id text-muted">text/template</code> syntax.
        Variables resolve against the matched event; see the rendered result on the right.
      </FieldDescription>
    </Field>
  );
}
