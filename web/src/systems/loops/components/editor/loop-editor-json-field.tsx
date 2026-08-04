import { useId, useState } from "react";
import { AlertTriangle } from "lucide-react";

import { FieldError, Textarea } from "@compozy/ui";

interface LoopEditorJsonFieldProps {
  value: unknown;
  disabled: boolean;
  onCommit: (parsed: unknown) => void;
  testId?: string;
  ariaLabel?: string;
  placeholder?: string;
  className?: string;
}

/**
 * A structured (object/array) field edited as JSON text and parsed back on change, so a scalar
 * edit never coerces the field to a string. Invalid JSON is surfaced, not saved.
 *
 * Local validation is **parseability only** — never semantics. Whether a tool exists, an event
 * kind is supported, or a payload matches its schema is daemon truth surfaced in the lint dock
 * (`effect_tool_unknown`, `effect_shape_invalid`); the editor must not invent a second verdict.
 */
export function LoopEditorJsonField({
  value,
  disabled,
  onCommit,
  testId,
  ariaLabel,
  placeholder,
  className = "min-h-18.5 resize-y text-form-input leading-relaxed",
}: LoopEditorJsonFieldProps) {
  const [text, setText] = useState(() =>
    value === undefined ? "" : JSON.stringify(value, null, 2)
  );
  const [error, setError] = useState<string | null>(null);
  const errorId = useId();
  return (
    <div>
      <Textarea
        aria-describedby={error ? errorId : undefined}
        aria-invalid={Boolean(error)}
        aria-label={ariaLabel}
        className={className}
        data-testid={testId}
        disabled={disabled}
        placeholder={placeholder}
        value={text}
        variant="mono"
        onChange={event => {
          const next = event.target.value;
          setText(next);
          if (next.trim() === "") {
            setError(null);
            onCommit(undefined);
            return;
          }
          try {
            const parsed = JSON.parse(next);
            setError(null);
            onCommit(parsed);
          } catch {
            setError("Invalid JSON — not saved");
          }
        }}
      />
      {error ? (
        <FieldError className="mt-1.5 flex items-center gap-1.5 text-form-hint" id={errorId}>
          <AlertTriangle aria-hidden="true" className="size-3" />
          {error}
        </FieldError>
      ) : null}
    </div>
  );
}
