import { Plus, X } from "lucide-react";

import { Button, Input, Label, NativeSelect, NativeSelectOption } from "@agh/ui";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { MonoTag } from "../mono-tag";
import { LoopReferenceInput } from "./loop-reference-input";

type Criterion = Record<string, unknown> & { id?: string; type?: string };

interface LoopEditorCriteriaProps {
  value: unknown;
  onChange: (criteria: Criterion[]) => void;
  suggestions?: readonly LoopReferenceSuggestion[];
  disabled?: boolean;
  allowedTypes?: readonly CriterionType[];
}

const CRITERION_TYPES = ["command", "agent-judge", "human", "extension"] as const;
type CriterionType = (typeof CRITERION_TYPES)[number];

function asCriteria(value: unknown): Criterion[] {
  return Array.isArray(value)
    ? (value.filter(item => item && typeof item === "object") as Criterion[])
    : [];
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function nextCriterionId(criteria: readonly Criterion[]): string {
  const maxSuffix = criteria.reduce((max, criterion) => {
    const match = /^criterion_(\d+)$/.exec(str(criterion.id));
    return match ? Math.max(max, Number(match[1])) : max;
  }, 0);
  return `criterion_${maxSuffix + 1}`;
}

function criterionForType(id: string, type: CriterionType): Criterion {
  switch (type) {
    case "command":
      return { id, type, check: "", expect: "exit_zero" };
    case "agent-judge":
      return { id, type, agent: "", rubric: "" };
    case "human":
      return { id, type, prompt: "" };
    case "extension":
      return { id, type, tool: "" };
  }
}

/**
 * The gate criteria list editor (design §4.6): one row per verdict criterion with the
 * per-type fields the DSL carries — command (check + expect), agent-judge (agent +
 * rubric), human (prompt), extension (tool). Editing writes the whole criteria array
 * back through the codec verbatim; the shared linter enforces that
 * `revise_until_clean` has a judge/human criterion (`verdict_policy_requires_judge`).
 */
export function LoopEditorCriteria({
  value,
  onChange,
  suggestions = [],
  disabled = false,
  allowedTypes = CRITERION_TYPES,
}: LoopEditorCriteriaProps) {
  const defaultType = allowedTypes[0];
  const allowedTypeSet = new Set(allowedTypes);
  const criteria = asCriteria(value).map(criterion => {
    const type = str(criterion.type) as CriterionType;
    return defaultType && !allowedTypeSet.has(type)
      ? { ...criterion, type: defaultType }
      : criterion;
  });
  const rowKeys = useLocalRowKeys(criteria, "criterion");

  const update = (index: number, patch: Record<string, unknown>) => {
    onChange(
      criteria.map((criterion, i) => (i === index ? { ...criterion, ...patch } : criterion))
    );
  };
  const remove = (index: number) => {
    rowKeys.remove(index);
    onChange(criteria.filter((_, i) => i !== index));
  };
  const add = () => {
    if (!defaultType) return;
    rowKeys.append();
    onChange([...criteria, criterionForType(nextCriterionId(criteria), defaultType)]);
  };

  return (
    <div className="flex flex-col gap-2" data-testid="loop-editor-criteria">
      {criteria.map((criterion, index) => (
        <div
          key={rowKeys.keys[index]}
          className="rounded-md border border-line-soft bg-canvas-soft p-2.5"
        >
          <div className="mb-2 flex items-center gap-2">
            <span className="font-mono text-mono-id text-fg-strong">
              {str(criterion.id) || "criterion"}
            </span>
            <MonoTag className="ml-auto rounded-xs bg-badge-fill px-1.5 py-0.5 text-pill-group-badge text-subtle">
              {str(criterion.type) || "command"}
            </MonoTag>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={disabled}
              onClick={() => remove(index)}
              aria-label={`Remove criterion ${str(criterion.id) || index + 1}`}
            >
              <X aria-hidden="true" className="size-3" />
            </Button>
          </div>
          <CriterionBody
            criterion={criterion}
            disabled={disabled}
            suggestions={suggestions}
            allowedTypes={allowedTypes}
            onChange={patch => update(index, patch)}
          />
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled || !defaultType}
        onClick={add}
        className="border-dashed"
        data-testid="loop-editor-criteria-add"
      >
        <Plus aria-hidden="true" className="size-3" />
        Add criterion
      </Button>
    </div>
  );
}

interface CriterionBodyProps {
  criterion: Criterion;
  disabled: boolean;
  suggestions: readonly LoopReferenceSuggestion[];
  onChange: (patch: Record<string, unknown>) => void;
  allowedTypes: readonly CriterionType[];
}

const fieldClass = "h-8 px-2.5 font-mono text-form-input";

function CriterionBody({
  criterion,
  disabled,
  suggestions,
  onChange,
  allowedTypes,
}: CriterionBodyProps) {
  const type = str(criterion.type);
  return (
    <div className="flex flex-col gap-2">
      <NativeSelect
        value={type}
        disabled={disabled}
        onChange={event => onChange({ type: event.target.value })}
        aria-label="Criterion type"
      >
        {allowedTypes.map(option => (
          <NativeSelectOption key={option} value={option}>
            {option}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      {type === "command" ? (
        <div className="flex gap-2">
          <div className="flex-1">
            <LoopReferenceInput
              value={str(criterion.check)}
              onChange={next => onChange({ check: next })}
              suggestions={suggestions}
              disabled={disabled}
              mono
              placeholder="{{ .inputs.verify_command }}"
              ariaLabel="Command check"
            />
          </div>
          <NativeSelect
            className="w-32 shrink-0"
            value={str(criterion.expect) || "exit_zero"}
            disabled={disabled}
            onChange={event => onChange({ expect: event.target.value })}
            aria-label="Expect"
          >
            <NativeSelectOption value="exit_zero">exit_zero</NativeSelectOption>
            <NativeSelectOption value="stdout_match">stdout_match</NativeSelectOption>
          </NativeSelect>
        </div>
      ) : null}
      {type === "agent-judge" ? (
        <>
          <Input
            className={fieldClass}
            value={str(criterion.agent)}
            disabled={disabled}
            placeholder="reviewer"
            onChange={event => onChange({ agent: event.target.value })}
            aria-label="Judge agent"
          />
          <LoopReferenceInput
            value={str(criterion.rubric)}
            onChange={next => onChange({ rubric: next })}
            suggestions={suggestions}
            disabled={disabled}
            multiline
            placeholder="Judge the diff against each task's acceptance…"
            ariaLabel="Rubric"
          />
        </>
      ) : null}
      {type === "human" ? (
        <LoopReferenceInput
          value={str(criterion.prompt)}
          onChange={next => onChange({ prompt: next })}
          suggestions={suggestions}
          disabled={disabled}
          mono
          placeholder="Approve the delivered work?"
          ariaLabel="Human prompt"
        />
      ) : null}
      {type === "extension" ? (
        <Input
          className={fieldClass}
          value={str(criterion.tool)}
          disabled={disabled}
          placeholder="ext__gate_check"
          onChange={event => onChange({ tool: event.target.value })}
          aria-label="Extension tool"
        />
      ) : null}
      <Label className="text-badge text-faint">
        id
        <Input
          className={`${fieldClass} mt-1`}
          value={str(criterion.id)}
          disabled={disabled}
          onChange={event => onChange({ id: event.target.value })}
          aria-label="Criterion id"
        />
      </Label>
    </div>
  );
}
