import { NativeSelect, NativeSelectOption } from "@compozy/ui";

import type { RawLoopNode } from "../../lib/codec";
import type { NodeFieldEdit } from "../../lib/loop-editor-draft";
import { waitModeEdits } from "../../lib/loop-node-wait-fields";
import type { LoopWaitMode, WaitModeFieldSpec } from "../../lib/loop-node-schema-types";

interface LoopEditorWaitModeProps {
  field: WaitModeFieldSpec;
  raw: RawLoopNode;
  disabled: boolean;
  onChangeFields: (edits: NodeFieldEdit[]) => void;
}

const WAIT_MODES: LoopWaitMode[] = ["for", "until", "event"];

/**
 * The `wait` discriminator switch. It is deliberately NOT a plain select field: the linter
 * rejects any extra `params` key and requires exactly one of for/until/event, so selecting a
 * mode emits all three edits at once (`waitModeEdits`) and the draft applies them in a single
 * transition (`setNodeFields`). No intermediate two-discriminator shape is ever observable.
 */
export function LoopEditorWaitMode({
  field,
  raw,
  disabled,
  onChangeFields,
}: LoopEditorWaitModeProps) {
  return (
    <div>
      <span className="mb-1.5 flex items-center gap-1.5 text-form-label font-medium text-fg-strong">
        {field.label}
        <span className="text-accent-strong" title="required">
          *
        </span>
      </span>
      <NativeSelect
        value={field.mode ?? ""}
        disabled={disabled}
        aria-label={field.label}
        data-testid={`loop-field-${field.key}`}
        onChange={event => onChangeFields(waitModeEdits(raw, event.target.value as LoopWaitMode))}
      >
        {field.mode === null ? (
          <NativeSelectOption value="" disabled>
            none declared
          </NativeSelectOption>
        ) : null}
        {WAIT_MODES.map(mode => (
          <NativeSelectOption key={mode} value={mode}>
            {mode}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      {field.hint ? (
        <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
      ) : null}
    </div>
  );
}
