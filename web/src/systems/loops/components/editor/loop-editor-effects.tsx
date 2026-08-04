import { Plus, X } from "lucide-react";

import { Button, NativeSelect, NativeSelectOption } from "@compozy/ui";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { LoopEditorJsonField } from "./loop-editor-json-field";
import { LoopReferenceInput } from "./loop-reference-input";

/** One `dsl.EffectSpec` row: exactly one of `{emit:{kind,payload?}}` or `{tool,with}`. */
type EffectRow = {
  emit?: { kind?: string; payload?: unknown };
  tool?: string;
  with?: unknown;
};

type EffectShape = "emit" | "tool";

interface LoopEditorEffectsProps {
  value: unknown;
  /** `undefined` clears the whole list path, so no `on_*: []` ever reaches the definition. */
  onChange: (effects: EffectRow[] | undefined) => void;
  suggestions?: readonly LoopReferenceSuggestion[];
  disabled?: boolean;
  testId: string;
  /** Human label of the owning trigger, used for accessible per-row names. */
  label: string;
}

const EMPTY_REFERENCE_SUGGESTIONS: readonly LoopReferenceSuggestion[] = [];

function asRows(value: unknown): EffectRow[] {
  return Array.isArray(value)
    ? (value.filter(item => item && typeof item === "object") as EffectRow[])
    : [];
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function shapeOf(row: EffectRow): EffectShape {
  return row.emit !== undefined ? "emit" : "tool";
}

/** Rebuilds the row so switching shape cannot retain fields from the other variant. */
function withShape(row: EffectRow, shape: EffectShape): EffectRow {
  if (shape === "emit") return { emit: { kind: str(row.tool) || str(row.emit?.kind) } };
  return { tool: str(row.tool) || str(row.emit?.kind) };
}

/** Effect-list editor. Rows are emit XOR tool; semantic validation stays daemon-owned. */
export function LoopEditorEffects({
  value,
  onChange,
  suggestions = EMPTY_REFERENCE_SUGGESTIONS,
  disabled = false,
  testId,
  label,
}: LoopEditorEffectsProps) {
  const rows = asRows(value);
  // Row ids are component-local: they stabilize editor instances without entering the loop DSL.
  const rowKeys = useLocalRowKeys(rows, "effect");

  const replace = (index: number, row: EffectRow) => {
    onChange(rows.map((entry, i) => (i === index ? row : entry)));
  };
  const remove = (index: number) => {
    rowKeys.remove(index);
    const next = rows.filter((_, i) => i !== index);
    onChange(next.length === 0 ? undefined : next);
  };
  const add = () => {
    rowKeys.append();
    onChange([...rows, { emit: { kind: "" } }]);
  };

  return (
    <div className="flex flex-col gap-2" data-testid={testId}>
      {rows.map((row, index) => {
        const shape = shapeOf(row);
        const rowName = `${label} effect ${index + 1}`;
        return (
          <div
            key={rowKeys.keys[index]}
            className="rounded-md border border-line-soft bg-canvas-soft p-2"
            data-testid={`${testId}-row`}
            data-shape={shape}
          >
            <div className="flex items-center gap-2">
              <span className="text-badge text-faint">shape</span>
              <div className="w-18 shrink-0">
                <NativeSelect
                  value={shape}
                  disabled={disabled}
                  onChange={event =>
                    replace(index, withShape(row, event.target.value as EffectShape))
                  }
                  aria-label={`${rowName} shape`}
                  data-testid={`${testId}-shape-${index}`}
                >
                  <NativeSelectOption value="emit">emit</NativeSelectOption>
                  <NativeSelectOption value="tool">tool</NativeSelectOption>
                </NativeSelect>
              </div>
              <span className="text-badge text-faint">{shape === "emit" ? "kind" : "tool"}</span>
              <div className="min-w-0 flex-1">
                <div>
                  <LoopReferenceInput
                    value={shape === "emit" ? str(row.emit?.kind) : str(row.tool)}
                    onChange={next =>
                      replace(
                        index,
                        shape === "emit"
                          ? { emit: { ...row.emit, kind: next } }
                          : { ...row, tool: next }
                      )
                    }
                    suggestions={suggestions}
                    disabled={disabled}
                    mono
                    placeholder={shape === "emit" ? "task_retrying" : "compozy__network_send"}
                    ariaLabel={`${rowName} ${shape === "emit" ? "event kind" : "tool id"}`}
                    testId={`${testId}-${shape === "emit" ? "kind" : "tool"}-${index}`}
                  />
                </div>
              </div>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                disabled={disabled}
                onClick={() => remove(index)}
                aria-label={`Remove ${rowName}`}
              >
                <X aria-hidden="true" className="size-3" />
              </Button>
            </div>
            <details
              className="mt-2"
              open={(shape === "emit" ? row.emit?.payload : row.with) !== undefined || undefined}
            >
              <summary className="cursor-pointer font-mono text-mono-id text-subtle">
                {shape === "emit" ? "payload" : "with"} · optional
              </summary>
              <div className="mt-1">
                <LoopEditorJsonField
                  key={shape}
                  value={shape === "emit" ? row.emit?.payload : row.with}
                  disabled={disabled}
                  placeholder={shape === "emit" ? "event payload" : "tool arguments"}
                  ariaLabel={`${rowName} ${shape === "emit" ? "payload" : "arguments"}`}
                  testId={`${testId}-${shape === "emit" ? "payload" : "with"}-${index}`}
                  className="min-h-14 resize-y text-form-input leading-relaxed"
                  onCommit={parsed =>
                    replace(
                      index,
                      shape === "emit"
                        ? {
                            emit: {
                              kind: str(row.emit?.kind),
                              ...(parsed === undefined ? {} : { payload: parsed }),
                            },
                          }
                        : {
                            tool: str(row.tool),
                            ...(parsed === undefined ? {} : { with: parsed }),
                          }
                    )
                  }
                />
              </div>
            </details>
          </div>
        );
      })}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={add}
        className="border-dashed"
        data-testid={`${testId}-add`}
      >
        <Plus aria-hidden="true" className="size-3" />
        Add effect
      </Button>
    </div>
  );
}
