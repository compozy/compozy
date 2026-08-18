import { useId } from "react";

import {
  Label,
  NativeSelect,
  NativeSelectOption,
  PillGroup,
  type PillGroupItem,
} from "@compozy/ui";

type LoopDiffMode = "generation" | "run";

export interface LoopRunDiffPickersProps {
  mode: LoopDiffMode;
  generations: readonly number[];
  baseGeneration: number | null;
  againstGeneration: number | null;

  runs: readonly { id: string; label: string }[];
  againstRunId: string;
  onModeChange: (mode: LoopDiffMode) => void;
  onBaseGenerationChange: (generation: number) => void;
  onAgainstGenerationChange: (generation: number) => void;
  onAgainstRunChange: (runId: string) => void;
}

interface GenerationSelectProps {
  generations: readonly number[];
  id: string;
  label: string;
  onChange: (generation: number) => void;
  testId: string;
  value: number | null;
}

const PICKER_CLASS = "w-full min-w-44 sm:w-44";

function GenerationSelect({
  generations,
  id,
  label,
  onChange,
  testId,
  value,
}: GenerationSelectProps) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Label className="eyebrow text-subtle" htmlFor={id}>
        {label}
      </Label>
      <NativeSelect
        className={PICKER_CLASS}
        data-testid={testId}
        disabled={generations.length === 0}
        id={id}
        onChange={event => onChange(Number(event.target.value))}
        size="sm"
        value={value === null ? "" : String(value)}
      >
        {value === null ? <NativeSelectOption value="">—</NativeSelectOption> : null}
        {generations.map(generation => (
          <NativeSelectOption key={generation} value={generation}>
            {`Generation ${generation}`}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  );
}

export function LoopRunDiffPickers({
  mode,
  generations,
  baseGeneration,
  againstGeneration,
  runs,
  againstRunId,
  onModeChange,
  onBaseGenerationChange,
  onAgainstGenerationChange,
  onAgainstRunChange,
}: LoopRunDiffPickersProps) {
  const fieldId = useId();
  const modeItems: PillGroupItem<LoopDiffMode>[] = [
    { value: "generation", label: "Generation", testId: "loop-diff-mode-generation" },
    {
      value: "run",
      label: "Run",
      disabled: runs.length === 0,
      testId: "loop-diff-mode-run",
    },
  ];
  return (
    <div className="flex flex-wrap items-end gap-x-4 gap-y-3" data-testid="loop-diff-pickers">
      <div className="flex flex-col gap-1.5">
        <span className="eyebrow text-subtle">Compare</span>
        <PillGroup
          aria-label="What to compare against"
          items={modeItems}
          onChange={onModeChange}
          value={mode}
        />
      </div>
      <GenerationSelect
        generations={generations}
        id={`${fieldId}-base`}
        label="Base"
        onChange={onBaseGenerationChange}
        testId="loop-diff-base-generation"
        value={baseGeneration}
      />
      {mode === "generation" ? (
        <GenerationSelect
          generations={generations}
          id={`${fieldId}-against`}
          label="Against"
          onChange={onAgainstGenerationChange}
          testId="loop-diff-against-generation"
          value={againstGeneration}
        />
      ) : (
        <div className="flex min-w-0 flex-col gap-1.5">
          <Label className="eyebrow text-subtle" htmlFor={`${fieldId}-run`}>
            Against
          </Label>
          <NativeSelect
            className={PICKER_CLASS}
            data-testid="loop-diff-against-run"
            disabled={runs.length === 0}
            id={`${fieldId}-run`}
            onChange={event => onAgainstRunChange(event.target.value)}
            size="sm"
            value={againstRunId}
          >
            {againstRunId === "" ? <NativeSelectOption value="">—</NativeSelectOption> : null}
            {runs.map(run => (
              <NativeSelectOption key={run.id} value={run.id}>
                {run.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      )}
    </div>
  );
}
