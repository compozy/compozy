import { useId, useRef } from "react";

import { cn, Input, Textarea } from "@agh/ui";

import { useReferenceAutocomplete } from "../../hooks/use-reference-autocomplete";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";

interface LoopReferenceInputProps {
  value: string;
  onChange: (value: string) => void;
  suggestions: readonly LoopReferenceSuggestion[];
  multiline?: boolean;
  mono?: boolean;
  placeholder?: string;
  invalid?: boolean;
  disabled?: boolean;
  /** CEL condition field: autocomplete triggers on a bare identifier, not `{{`. */
  cel?: boolean;
  ariaLabel?: string;
  testId?: string;
}

type ReferenceFieldElement = HTMLInputElement | HTMLTextAreaElement;

/**
 * A template-string input with the ADR-020 `{{ }}` reference picker: typing `{{ .`
 * surfaces the loop namespace (inputs, upstream node outputs, item/index, trigger,
 * generation) filtered by the partial path; selecting one inserts `{{ .path }}`. The
 * autocomplete state lives in `useReferenceAutocomplete`, so this stays presentational.
 */
export function LoopReferenceInput({
  value,
  onChange,
  suggestions,
  multiline = false,
  mono = false,
  placeholder,
  invalid = false,
  disabled = false,
  cel = false,
  ariaLabel,
  testId,
}: LoopReferenceInputProps) {
  const fieldRef = useRef<ReferenceFieldElement>(null);
  const listboxId = useId();
  const auto = useReferenceAutocomplete(fieldRef, onChange, suggestions, cel);
  const setFieldRef = (element: ReferenceFieldElement | null) => {
    fieldRef.current = element;
  };

  const inputClass = cn(
    "px-2.5 text-form-input",
    mono && "font-mono",
    multiline ? "min-h-18.5 resize-y leading-relaxed" : "h-8"
  );

  const activeOptionId =
    auto.matches.length > 0 ? `${listboxId}-option-${auto.activeIndex}` : undefined;

  const shared = {
    value,
    disabled,
    placeholder,
    onChange: auto.onChange,
    onKeyDown: auto.onKeyDown,
    onKeyUp: auto.onCaretMove,
    onClick: auto.onCaretMove,
    onBlur: auto.onBlur,
    className: inputClass,
    "aria-label": ariaLabel,
    "aria-autocomplete": "list" as const,
    "aria-controls": listboxId,
    "aria-expanded": auto.matches.length > 0,
    "aria-haspopup": "listbox" as const,
    "aria-invalid": invalid || undefined,
    "aria-activedescendant": activeOptionId,
    role: "combobox" as const,
    "data-testid": testId,
  };

  return (
    <div className="relative">
      {multiline ? (
        <Textarea ref={setFieldRef} variant={mono ? "mono" : "default"} {...shared} />
      ) : (
        <Input type="text" ref={setFieldRef} {...shared} />
      )}
      {auto.matches.length > 0 ? (
        <ul
          aria-label="Reference suggestions"
          className="absolute z-20 mt-1 max-h-52 w-full overflow-y-auto rounded-md border border-line-strong bg-elevated py-1 shadow-lg"
          data-testid="loop-reference-suggestions"
          id={listboxId}
          role="listbox"
        >
          {auto.matches.map((match, index) => (
            <li
              aria-selected={index === auto.activeIndex}
              className={cn(
                "flex w-full cursor-default items-center justify-between gap-3 px-2.5 py-1 text-left hover:bg-row-hover",
                index === auto.activeIndex && "bg-row-hover"
              )}
              data-active={index === auto.activeIndex ? "true" : "false"}
              id={`${listboxId}-option-${index}`}
              key={match.path}
              // Keep focus on the field so the caret survives the insert.
              onMouseDown={event => {
                event.preventDefault();
                auto.select(match.path);
              }}
              onMouseEnter={() => auto.setActiveIndex(index)}
              role="option"
            >
              <span className="font-mono text-mono-id text-fg-strong">{match.path}</span>
              <span className="truncate text-badge text-subtle">{match.detail}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

export type { LoopReferenceSuggestion };
