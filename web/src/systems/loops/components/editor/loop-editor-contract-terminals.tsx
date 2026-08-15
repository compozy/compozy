import {
  LOOP_TERMINAL_REACTIONS,
  LOOP_TERMINAL_REACTIONS_HINT,
} from "../../lib/loop-contract-terminal-fields";
import { getAtPath } from "../../lib/loop-editor-draft";
import type { FieldPath } from "../../lib/loop-node-schema";
import type { LoopContract } from "../../types";
import { LoopEditorEffects } from "./loop-editor-effects";
import { LoopEditorFold } from "./loop-editor-fold";

interface LoopEditorContractTerminalsProps {
  contract: LoopContract;
  disabled: boolean;
  onChangePath: (path: FieldPath, value: unknown) => void;
}

/**
 * The seven contract terminal reactions (ADR-010 §1), each a plain effect list firing exactly
 * once per run on the resulting outcome, including `on_canceled`.
 */
export function LoopEditorContractTerminals({
  contract,
  disabled,
  onChangePath,
}: LoopEditorContractTerminalsProps) {
  return (
    <LoopEditorFold defaultOpen label="Terminal reactions" subLabel="on_done … on_canceled">
      <div className="flex flex-col gap-3" data-testid="loop-editor-contract-terminals">
        <p className="text-form-hint leading-relaxed text-subtle">{LOOP_TERMINAL_REACTIONS_HINT}</p>
        {LOOP_TERMINAL_REACTIONS.map(field => (
          <div key={field.key}>
            <span className="mb-1.5 block font-mono text-mono-id text-fg-strong">
              {field.label}
            </span>
            <LoopEditorEffects
              value={getAtPath(contract, field.path)}
              disabled={disabled}
              label={field.label}
              testId={`loop-contract-${field.key}`}
              onChange={effects => onChangePath(field.path, effects)}
            />
          </div>
        ))}
      </div>
    </LoopEditorFold>
  );
}
