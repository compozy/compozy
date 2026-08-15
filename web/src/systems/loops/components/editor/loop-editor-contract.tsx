import { Field, FieldDescription, FieldHeader, FieldLabel, Textarea } from "@compozy/ui";

import type { EditableLoopContractField } from "../../lib/loop-editor-definition";
import type { FieldPath } from "../../lib/loop-node-schema";
import type { LoopContract } from "../../types";
import { LoopEditorContractTerminals } from "./loop-editor-contract-terminals";

interface LoopEditorContractProps {
  contract: LoopContract;
  disabled: boolean;
  onChange: (field: EditableLoopContractField, value: string) => void;
  onChangePath: (path: FieldPath, value: unknown) => void;
}

/** Authoring controls for the Loop-level outcome contract, separate from graph-node fields. */
export function LoopEditorContract({
  contract,
  disabled,
  onChange,
  onChangePath,
}: LoopEditorContractProps) {
  return (
    <section className="bg-canvas px-4 py-3.5" data-testid="loop-editor-contract">
      <p className="mb-3 text-form-hint leading-relaxed text-subtle">
        Define the outcome this Loop should reach.
      </p>
      <div className="flex flex-col gap-3.5">
        <Field>
          <FieldHeader>
            <FieldLabel htmlFor="loop-contract-goal">Goal</FieldLabel>
            <span className="text-badge font-normal text-faint">optional</span>
          </FieldHeader>
          <Textarea
            id="loop-contract-goal"
            rows={2}
            value={contract.goal ?? ""}
            disabled={disabled}
            onChange={event => onChange("goal", event.target.value)}
            className="min-h-16 resize-y"
          />
          <FieldDescription>Clear this field to publish a goal-less Loop.</FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="loop-contract-definition-of-done">Definition of done</FieldLabel>
          <Textarea
            id="loop-contract-definition-of-done"
            rows={3}
            value={contract.definition_of_done}
            disabled={disabled}
            onChange={event => onChange("definition_of_done", event.target.value)}
            className="min-h-20 resize-y"
          />
        </Field>
        <LoopEditorContractTerminals
          contract={contract}
          disabled={disabled}
          onChangePath={onChangePath}
        />
      </div>
    </section>
  );
}
