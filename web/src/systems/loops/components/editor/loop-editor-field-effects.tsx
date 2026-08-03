import type { FieldPath } from "../../lib/loop-node-schema";
import type { EffectsFieldSpec } from "../../lib/loop-node-schema-types";
import { getAtPath } from "../../lib/loop-editor-draft";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { LoopEditorEffects } from "./loop-editor-effects";

interface LoopEditorEffectsFieldProps {
  field: EffectsFieldSpec;
  raw: Record<string, unknown>;
  suggestions: readonly LoopReferenceSuggestion[];
  disabled: boolean;
  onChange: (path: FieldPath, value: unknown) => void;
}

/**
 * The inspector wrapper for one effect list — label, list, hint. Kept beside the list editor
 * so `loop-editor-field.tsx` only gains a dispatch line instead of a third form grammar.
 */
export function LoopEditorEffectsField({
  field,
  raw,
  suggestions,
  disabled,
  onChange,
}: LoopEditorEffectsFieldProps) {
  return (
    <div>
      <span className="mb-1.5 block font-mono text-mono-id text-fg-strong">{field.label}</span>
      <LoopEditorEffects
        value={getAtPath(raw, field.path)}
        suggestions={suggestions}
        disabled={disabled}
        label={field.label}
        testId={`loop-field-${field.key}`}
        onChange={effects => onChange(field.path, effects)}
      />
      {field.hint ? (
        <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
      ) : null}
    </div>
  );
}
