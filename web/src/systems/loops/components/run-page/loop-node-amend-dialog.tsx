import { useState } from "react";
import { Info } from "lucide-react";

import {
  ConfirmDialog,
  Eyebrow,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Textarea,
} from "@compozy/ui";

import type { LoopControlAnswer } from "../../lib/loop-node-controls";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { loopNodeStateStrip } from "../../lib/loop-node-verb-copy";
import { LOOP_NODE_VERB_ICON_TONE, LOOP_NODE_VERB_ICONS } from "../../lib/loop-node-verb-icons";
import {
  checkLoopRequestFields,
  checkLoopRequestJson,
  isLoopRequestFieldSchema,
  type LoopRequestField,
  loopRequestFields,
  loopRequestFieldSeed,
} from "../../lib/loop-request-payload";
import { LoopControlAnswerAlert } from "./loop-control-answer-alert";

export interface LoopNodeAmendDialogProps {
  open: boolean;
  node: LoopNodeLifecycle | null;

  originalOutput: unknown;

  outputSchema: unknown;
  isPending?: boolean;

  fieldErrors?: Readonly<Record<string, string>>;

  answer?: LoopControlAnswer | null;
  onOpenChange: (open: boolean) => void;
  onConfirm: (input: { payload: Record<string, unknown>; reason: string }) => void;
}

type OpenLoopNodeAmendDialogProps = Omit<LoopNodeAmendDialogProps, "node"> & {
  node: LoopNodeLifecycle;
};

const DIALOG_WIDTH = { className: "sm:max-w-(--width-modal-md)" };
const EDITOR_LABEL_ID = "loop-amend-editor-label";
const ORIGINAL_LABEL_ID = "loop-amend-original-label";
const BOOLEAN_CHOICES: string[] = ["true", "false"];
const MICRO_FIELD_LIMIT = 3;

export function LoopNodeAmendDialog({ node, ...props }: LoopNodeAmendDialogProps) {
  if (!node) return null;

  const formKey = [node.nodeId, node.generation, node.revision].join(":");
  return <LoopNodeAmendDialogForm key={formKey} {...props} node={node} />;
}

function LoopNodeAmendDialogForm({
  open,
  node,
  originalOutput,
  outputSchema,
  isPending,
  fieldErrors,
  answer,
  onOpenChange,
  onConfirm,
}: OpenLoopNodeAmendDialogProps) {
  const fields = loopRequestFields(outputSchema);
  const structured = isLoopRequestFieldSchema(outputSchema);
  const [values, setValues] = useState(() => loopRequestFieldSeed(fields, originalOutput));
  const [rawPayload, setRawPayload] = useState(() => rawSeed(originalOutput));
  const [reason, setReason] = useState("");
  const check = structured
    ? checkLoopRequestFields(fields, values)
    : checkLoopRequestJson(rawPayload, outputSchema);

  const errors: Record<string, string> = { ...fieldErrors, ...check.errors };
  const disabled = Boolean(isPending);
  return (
    <ConfirmDialog
      cancelLabel="Cancel"
      confirmButtonProps={{ "data-testid": "loop-amend-confirm", disabled: disabled || !check.ok }}
      confirmLabel="Amend"
      contentProps={{ "data-testid": "loop-node-amend-dialog", ...DIALOG_WIDTH }}
      description="Correct the effective output. The recorded original stays in history and is never rewritten."
      eyebrow="Node"
      footNote={
        <>
          <Info aria-hidden="true" />
          <span>{amendMicro(node, fields, structured)}</span>
        </>
      }
      icon={LOOP_NODE_VERB_ICONS.amend}
      iconTone={LOOP_NODE_VERB_ICON_TONE.amend}
      isPending={isPending}
      note={loopNodeStateStrip(node)}
      noteTone="info"
      onConfirm={() => onConfirm({ payload: check.payload ?? {}, reason })}
      onOpenChange={onOpenChange}
      open={open}
      title="Amend output"
      tone="accent"
      body={
        <>
          {answer ? <LoopControlAnswerAlert answer={answer} /> : null}
          <div className="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2">
            <section className="flex min-w-0 flex-col gap-1.5">
              <Eyebrow className="text-subtle" id={ORIGINAL_LABEL_ID}>
                Original
              </Eyebrow>
              <pre
                aria-labelledby={ORIGINAL_LABEL_ID}
                className="max-h-64 overflow-auto rounded-md border border-line-soft bg-canvas-tint px-3 py-2.5 font-mono text-mono-id leading-relaxed break-words whitespace-pre-wrap text-subtle focus-visible:shadow-focus-ring focus-visible:outline-none"
                data-testid="loop-amend-original"
                tabIndex={0}
              >
                {readableOutput(originalOutput)}
              </pre>
            </section>
            <section className="flex min-w-0 flex-col gap-1.5">
              <Eyebrow className="text-subtle" id={EDITOR_LABEL_ID}>
                Amended
              </Eyebrow>
              {structured ? (
                <div className="flex min-w-0 flex-col gap-2.5">
                  {fields.map(field => (
                    <AmendField
                      disabled={disabled}
                      error={errors[field.name]}
                      field={field}
                      key={field.name}
                      onChange={next => setValues(current => ({ ...current, [field.name]: next }))}
                      value={values[field.name] ?? ""}
                    />
                  ))}
                </div>
              ) : (
                <Field data-invalid={errors.payload !== undefined || undefined}>
                  <Textarea
                    aria-invalid={errors.payload !== undefined || undefined}
                    aria-labelledby={EDITOR_LABEL_ID}
                    data-testid="loop-amend-raw-payload"
                    disabled={disabled}
                    id="loop-amend-raw-payload"
                    onChange={event => setRawPayload(event.target.value)}
                    rows={8}
                    value={rawPayload}
                    variant="mono"
                  />
                  {errors.payload ? (
                    <FieldError data-testid="loop-amend-field-error-payload">
                      {errors.payload}
                    </FieldError>
                  ) : null}
                </Field>
              )}
            </section>
          </div>
          <Field>
            <FieldLabel htmlFor="loop-amend-reason">
              Reason <span className="text-muted">optional</span>
            </FieldLabel>
            <Textarea
              data-testid="loop-amend-reason"
              disabled={disabled}
              id="loop-amend-reason"
              onChange={event => setReason(event.target.value)}
              placeholder="Why this correction"
              rows={2}
              value={reason}
            />
          </Field>
        </>
      }
    />
  );
}

interface AmendFieldProps {
  field: LoopRequestField;
  value: string;
  error?: string;
  disabled: boolean;
  onChange: (value: string) => void;
}

function AmendField({ field, value, error, disabled, onChange }: AmendFieldProps) {
  const id = `loop-amend-field-${field.name}`;
  return (
    <Field data-invalid={error !== undefined || undefined}>
      <FieldLabel htmlFor={id}>
        {field.name}
        <span className="font-mono text-mono-id text-faint">{field.type}</span>
      </FieldLabel>
      <AmendFieldControl
        disabled={disabled}
        field={field}
        id={id}
        invalid={error !== undefined}
        onChange={onChange}
        value={value}
      />
      {field.description ? <FieldDescription>{field.description}</FieldDescription> : null}
      {error ? (
        <FieldError data-testid={`loop-amend-field-error-${field.name}`}>{error}</FieldError>
      ) : null}
    </Field>
  );
}

interface AmendFieldControlProps extends Omit<AmendFieldProps, "error"> {
  id: string;
  invalid: boolean;
}

function AmendFieldControl({
  field,
  value,
  invalid,
  disabled,
  id,
  onChange,
}: AmendFieldControlProps) {
  const shared = {
    "aria-invalid": invalid || undefined,
    "data-testid": id,
    disabled,
    id,
    value,
  };
  if (field.type === "json") {
    return (
      <Textarea
        {...shared}
        onChange={event => onChange(event.target.value)}
        rows={3}
        variant="mono"
      />
    );
  }
  const choices = field.type === "boolean" ? BOOLEAN_CHOICES : field.options;
  if (choices.length === 0) {
    return <Input {...shared} onChange={event => onChange(event.target.value)} />;
  }
  return (
    <NativeSelect {...shared} className="w-full" onChange={event => onChange(event.target.value)}>
      {field.required && choices.includes(value) ? null : (
        <NativeSelectOption value="">Not set</NativeSelectOption>
      )}
      {choices.map(choice => (
        <NativeSelectOption key={choice} value={choice}>
          {choice}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  );
}

function readableOutput(value: unknown): string {
  if (value === undefined || value === null) return "This cell recorded no output.";
  if (typeof value === "object" && !Array.isArray(value)) {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) return "{}";
    return entries.map(([key, entry]) => `${key}: ${JSON.stringify(entry)}`).join("\n");
  }
  return JSON.stringify(value, null, 2) ?? "";
}

function rawSeed(value: unknown): string {
  if (value === undefined || value === null) return "";
  return JSON.stringify(value, null, 2) ?? "";
}

function amendMicro(
  node: LoopNodeLifecycle,
  fields: readonly LoopRequestField[],
  structured: boolean
): string {
  if (!structured) return `gen ${node.generation} · rev ${node.revision} · output_shape raw json`;
  const shown = fields.slice(0, MICRO_FIELD_LIMIT).map(field => field.name);
  const remaining = fields.length - shown.length;
  const names = remaining > 0 ? `${shown.join(", ")} +${remaining}` : shown.join(", ");
  return `gen ${node.generation} · rev ${node.revision} · output_shape {${names}}`;
}
