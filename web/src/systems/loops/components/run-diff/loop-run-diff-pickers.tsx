import { useId } from "react";

import {
  Label,
  PillGroup,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  type PillGroupItem,
} from "@compozy/ui";

import { LoopStatusPill } from "../loop-status-pill";

type LoopDiffMode = "generation" | "run";

export interface LoopRunDiffPickersProps {
  mode: LoopDiffMode;
  generations: readonly number[];
  baseGeneration: number | null;
  againstGeneration: number | null;

  runs: readonly { id: string; label: string }[];
  againstRunId: string;
  /** Resolved base-side status; rendered only once the diff view model exists. */
  baseStatus?: string;
  /** Resolved against-side status; rendered only once the diff view model exists. */
  againstStatus?: string;
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
  status?: string;
  testId: string;
  value: number | null;
}

const PICKER_CLASS = "w-full min-w-44 sm:w-44";

function GenerationSelect({
  generations,
  id,
  label,
  onChange,
  status,
  testId,
  value,
}: GenerationSelectProps) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Label className="eyebrow text-subtle" htmlFor={id}>
        {label}
      </Label>
      <div className="flex min-w-0 flex-wrap items-center gap-2">
        <Select
          disabled={generations.length === 0}
          onValueChange={next => {
            if (typeof next === "string" && next !== "") onChange(Number(next));
          }}
          value={value === null ? "" : String(value)}
        >
          <SelectTrigger className={PICKER_CLASS} data-testid={testId} id={id} size="sm">
            <SelectValue>{value === null ? "—" : `Generation ${value}`}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {generations.map(generation => (
              <SelectItem key={generation} value={String(generation)}>
                {`Generation ${generation}`}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {status ? <LoopStatusPill size="xs" status={status} /> : null}
      </div>
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
  baseStatus,
  againstStatus,
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
  const againstRunLabel =
    againstRunId === "" ? "—" : (runs.find(run => run.id === againstRunId)?.label ?? againstRunId);
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
        status={baseStatus}
        testId="loop-diff-base-generation"
        value={baseGeneration}
      />
      {mode === "generation" ? (
        <GenerationSelect
          generations={generations}
          id={`${fieldId}-against`}
          label="Against"
          onChange={onAgainstGenerationChange}
          status={againstStatus}
          testId="loop-diff-against-generation"
          value={againstGeneration}
        />
      ) : (
        <div className="flex min-w-0 flex-col gap-1.5">
          <Label className="eyebrow text-subtle" htmlFor={`${fieldId}-run`}>
            Against
          </Label>
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <Select
              disabled={runs.length === 0}
              onValueChange={next => {
                if (typeof next === "string") onAgainstRunChange(next);
              }}
              value={againstRunId}
            >
              <SelectTrigger
                className={PICKER_CLASS}
                data-testid="loop-diff-against-run"
                id={`${fieldId}-run`}
                size="sm"
              >
                <SelectValue>{againstRunLabel}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {runs.map(run => (
                  <SelectItem key={run.id} value={run.id}>
                    {run.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {againstStatus ? <LoopStatusPill size="xs" status={againstStatus} /> : null}
          </div>
        </div>
      )}
    </div>
  );
}
