import { PillGroup } from "@compozy/ui";

import type { RawLoopNode } from "../../lib/codec";
import type { NodeFieldEdit } from "../../lib/loop-editor-draft";
import { environmentModeEdits } from "../../lib/loop-node-environment-fields";
import {
  LOOP_ENVIRONMENT_MODE_LABELS,
  LOOP_ENVIRONMENT_MODES,
  type EnvironmentFieldSpec,
} from "../../lib/loop-node-schema-types";
import type { LoopEnvironmentMode } from "../../types";

interface LoopEditorEnvironmentFieldProps {
  field: EnvironmentFieldSpec;
  raw: RawLoopNode;
  disabled: boolean;
  onChangeFields: (edits: NodeFieldEdit[]) => void;
}

const INHERIT_VALUE = "__inherit__";

type EnvironmentChoice = LoopEnvironmentMode | typeof INHERIT_VALUE;

/**
 * The single execution-environment control on agent-executing nodes.
 *
 * Like the `wait` discriminator it is not a plain select: each mode forbids a
 * different companion key, so a mode change emits every edit at once and the
 * draft applies them in one transition — no rejected intermediate shape is ever
 * observable. Choosing Inherit clears the descriptor entirely, which is what
 * "follow the loop default" means on the wire.
 */
export function LoopEditorEnvironmentField({
  field,
  raw,
  disabled,
  onChangeFields,
}: LoopEditorEnvironmentFieldProps) {
  const items = [
    { value: INHERIT_VALUE as EnvironmentChoice, label: field.inheritLabel, disabled },
    ...LOOP_ENVIRONMENT_MODES.map(mode => ({
      value: mode as EnvironmentChoice,
      label: LOOP_ENVIRONMENT_MODE_LABELS[mode],
      disabled,
    })),
  ];

  function handleChange(next: EnvironmentChoice) {
    onChangeFields(environmentModeEdits(raw, next === INHERIT_VALUE ? null : next));
  }

  return (
    <div data-mode={field.mode ?? "inherit"} data-slot="loop-node-environment-field">
      <span className="mb-1.5 flex items-center gap-1.5 text-form-label font-medium text-fg-strong">
        {field.label}
      </span>
      <PillGroup
        aria-label={field.label}
        data-testid={`loop-field-${field.key}`}
        items={items}
        onChange={handleChange}
        size="sm"
        value={field.mode ?? INHERIT_VALUE}
      />
      {field.hint ? (
        <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
      ) : null}
    </div>
  );
}
