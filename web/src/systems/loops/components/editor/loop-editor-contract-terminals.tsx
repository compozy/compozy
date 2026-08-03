import {
  LOOP_TERMINAL_REACTIONS,
  LOOP_TERMINAL_REACTIONS_HINT,
} from "../../lib/loop-contract-terminal-fields";
import { getAtPath } from "../../lib/loop-editor-draft";
import type { FieldPath } from "../../lib/loop-node-schema";
import type { LoopContract } from "../../types";
import { LoopEditorEffects } from "./loop-editor-effects";

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
    <details className="rounded-md border border-line-soft bg-canvas-soft" open>
      <summary className="flex cursor-pointer items-center gap-2 px-3 py-2.5 text-form-label font-medium text-fg-strong">
        Terminal reactions
        <span className="text-badge font-normal text-faint">on_done … on_canceled</span>
      </summary>
      <div className="flex flex-col gap-3 px-3 pb-3" data-testid="loop-editor-contract-terminals">
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
    </details>
  );
}
