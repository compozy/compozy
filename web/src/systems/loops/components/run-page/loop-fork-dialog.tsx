import { useState } from "react";
import { GitFork } from "lucide-react";

import { ConfirmDialog, Label, RadioCard, Textarea } from "@compozy/ui";

import {
  initialRunInputs,
  serializeRunInputs,
  missingRequiredInputs,
} from "../../lib/loop-run-form";
import type { LoopInputSchema } from "../../types";
import { LoopRunInputField } from "../run-form/loop-run-input-field";

export interface LoopForkDialogProps {
  open: boolean;
  loopName: string;

  generations: readonly number[];
  defaultGeneration: number | null;

  inputSchema?: LoopInputSchema;

  sourceInputs: Readonly<Record<string, unknown>>;
  isPending?: boolean;

  fieldErrors?: Readonly<Record<string, string>>;

  blockedReason?: string;
  onOpenChange: (open: boolean) => void;
  onSubmit: (input: {
    generation: number;
    inputs: Record<string, unknown>;
    reason: string;
  }) => void;
}

type OpenLoopForkDialogProps = Omit<LoopForkDialogProps, "open">;

const DIALOG_WIDTH = { className: "sm:max-w-(--width-modal-sm)" };

const FORK_DESCRIPTION =
  "Starts a new run seeded from this generation. The source run is untouched.";

function forkInputs(
  schema: LoopInputSchema | undefined,
  sourceInputs: Readonly<Record<string, unknown>>
): Record<string, unknown> {
  return { ...initialRunInputs(schema), ...sourceInputs };
}

function matchesSource(
  inputs: Record<string, unknown>,
  sourceInputs: Readonly<Record<string, unknown>>
): boolean {
  const keys = new Set([...Object.keys(inputs), ...Object.keys(sourceInputs)]);
  for (const key of keys) {
    if (JSON.stringify(inputs[key]) !== JSON.stringify(sourceInputs[key])) return false;
  }
  return true;
}

export function LoopForkDialog({ open, ...props }: LoopForkDialogProps) {
  if (!open) return null;
  return <LoopForkDialogForm key={`${props.loopName}:${props.defaultGeneration}`} {...props} />;
}

function LoopForkDialogForm({
  loopName,
  generations,
  defaultGeneration,
  inputSchema,
  sourceInputs,
  isPending,
  fieldErrors,
  blockedReason,
  onOpenChange,
  onSubmit,
}: OpenLoopForkDialogProps) {
  const [generation, setGeneration] = useState<number | null>(
    () => defaultGeneration ?? generations[0] ?? null
  );
  const [inputs, setInputs] = useState<Record<string, unknown>>(() =>
    forkInputs(inputSchema, sourceInputs)
  );
  const [reason, setReason] = useState("");
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const entries = inputSchema ? Object.entries(inputSchema) : [];
  const missing = new Set(missingRequiredInputs(inputSchema, inputs));
  const blocked = Boolean(blockedReason);
  const isReplay = entries.length > 0 && matchesSource(inputs, sourceInputs);

  function errorFor(name: string): string | undefined {
    if (fieldErrors?.[name]) return fieldErrors[name];
    if (submitAttempted && missing.has(name)) return `${name} is required to run this loop.`;
    return undefined;
  }

  function handleConfirm() {
    if (blocked || isPending) return;
    if (generation === null || missing.size > 0) {
      setSubmitAttempted(true);
      return;
    }
    onSubmit({ generation, inputs: serializeRunInputs(inputSchema, inputs), reason });
  }

  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "loop-fork-submit", disabled: blocked || isPending }}
      confirmLabel="Start fork"
      contentProps={{ "data-testid": "loop-fork-dialog", ...DIALOG_WIDTH }}
      description={FORK_DESCRIPTION}
      eyebrow="Run"
      footNote={
        <>
          <GitFork aria-hidden="true" />
          <span>
            {generation === null
              ? `source ${loopName}`
              : `source ${loopName} · generation ${generation}`}
          </span>
        </>
      }
      icon={GitFork}
      iconTone="neutral"
      isPending={isPending}
      note={
        blockedReason ? (
          <>
            <span>This generation cannot be forked. Nothing was started.</span>
            <span className="mt-1 block font-mono text-mono-id break-words text-subtle">
              {blockedReason}
            </span>
          </>
        ) : undefined
      }
      noteProps={{ "data-testid": "loop-fork-blocked" }}
      noteTone="neutral"
      onConfirm={handleConfirm}
      onOpenChange={onOpenChange}
      open
      title="Fork from here"
      tone="accent"
      body={
        blocked ? null : (
          <>
            <fieldset className="flex flex-col gap-1.5" data-testid="loop-fork-generation">
              <legend className="eyebrow mb-1.5 text-subtle">Generation</legend>
              {generations.map(value => (
                <RadioCard
                  badge={
                    value === defaultGeneration ? (
                      <span className="font-mono text-mono-id text-faint">inspected</span>
                    ) : undefined
                  }
                  data-testid={`loop-fork-generation-${value}`}
                  disabled={isPending}
                  key={value}
                  onSelect={() => setGeneration(value)}
                  selected={value === generation}
                  title={`Generation ${value}`}
                />
              ))}
            </fieldset>
            {isReplay ? (
              <p className="rounded-md bg-info-tint px-3 py-2 text-form-hint leading-relaxed text-fg">
                Nothing is overridden — this replays the generation as a new run, and lineage still
                records the fork.
              </p>
            ) : null}
            {entries.length > 0 ? (
              <div className="flex flex-col gap-3">
                <span className="eyebrow text-subtle">Inputs</span>
                {entries.map(([name, field]) => (
                  <div data-testid={`loop-fork-input-${name}`} key={name}>
                    <LoopRunInputField
                      disabled={isPending}
                      error={errorFor(name)}
                      field={field}
                      name={name}
                      onChange={value => setInputs(current => ({ ...current, [name]: value }))}
                      value={inputs[name]}
                    />
                  </div>
                ))}
              </div>
            ) : null}
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="loop-fork-reason">
                Reason <span className="text-muted">optional</span>
              </Label>
              <Textarea
                data-testid="loop-fork-reason"
                disabled={isPending}
                id="loop-fork-reason"
                onChange={event => setReason(event.target.value)}
                rows={3}
                value={reason}
              />
            </div>
          </>
        )
      }
    />
  );
}
