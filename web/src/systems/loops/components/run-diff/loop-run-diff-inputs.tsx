import { TextCursorInput } from "lucide-react";

import { Pill, Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@compozy/ui";

import { LOOP_DIFF_CHANGE_LABEL, LOOP_DIFF_CHANGE_TONE } from "../../lib/loop-request-vocabulary";
import type { LoopDiffInputRowView } from "../../lib/loop-run-diff-model";
import { LoopRunDiffValue } from "./loop-run-diff-row";

export interface LoopRunDiffInputsProps {
  inputs: readonly LoopDiffInputRowView[];
}

export function LoopRunDiffInputs({ inputs }: LoopRunDiffInputsProps) {
  if (inputs.length === 0) return null;
  return (
    <div
      className="overflow-hidden rounded-md border border-line bg-canvas-soft"
      data-testid="loop-diff-inputs"
    >
      <div className="flex items-center gap-1.5 border-b border-line-soft px-3 py-2">
        <TextCursorInput aria-hidden="true" className="size-3 text-subtle" />
        <h3 className="eyebrow text-subtle">Inputs</h3>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Input</TableHead>
            <TableHead>Base</TableHead>
            <TableHead>Against</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {inputs.map(row => (
            <TableRow
              data-changed={row.changed ? "true" : undefined}
              data-testid={`loop-diff-input-${row.key}`}
              key={row.key}
            >
              <TableCell className="font-mono text-mono-id text-fg-strong">{row.key}</TableCell>
              <TableCell className="whitespace-normal">
                <LoopRunDiffValue value={row.base} />
              </TableCell>
              <TableCell className="whitespace-normal">
                <span className="flex flex-wrap items-center gap-2">
                  <LoopRunDiffValue value={row.against} />
                  {row.changed ? (
                    <Pill size="sm" tone={LOOP_DIFF_CHANGE_TONE.changed}>
                      {LOOP_DIFF_CHANGE_LABEL.changed}
                    </Pill>
                  ) : null}
                </span>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
