import type { ComponentType } from "react";

import type { RawLoopNode } from "../../lib/codec";
import type { FieldSpec } from "../../lib/loop-node-schema-types";
import { LoopEditorEnvironmentField } from "./loop-editor-environment-field";
import {
  LoopEditorCriteriaField,
  LoopEditorEventsField,
  LoopEditorHintField,
  LoopEditorNumberField,
  LoopEditorSelectField,
  LoopEditorStaticField,
  LoopEditorSwitchField,
  LoopEditorTextField,
  type LoopEditorFieldControlProps,
  type LoopEditorFieldProps,
} from "./loop-editor-field-controls";
import { LoopEditorEffectsField } from "./loop-editor-field-effects";
import { LoopEditorFold } from "./loop-editor-fold";
import { LoopEditorReviewField } from "./loop-editor-review-field";
import { LoopEditorRoute } from "./loop-editor-route";
import { LoopEditorStrategyField } from "./loop-editor-strategy-field";
import { LoopEditorWaitMode } from "./loop-editor-wait-mode";

type FieldType = FieldSpec["type"];

/** The descriptor variant whose `type` admits `K` (text and textarea share one spec). */
type FieldOfType<K extends FieldType, F extends FieldSpec = FieldSpec> = F extends {
  type: infer T;
}
  ? K extends T
    ? F
    : never
  : never;

type FieldControls = {
  [K in FieldType]: ComponentType<LoopEditorFieldControlProps<FieldOfType<K>>>;
};

function WaitModeControl({
  field,
  raw,
  disabled,
  onChangeFields,
}: LoopEditorFieldControlProps<FieldOfType<"wait-mode">>) {
  return (
    <LoopEditorWaitMode
      field={field}
      raw={raw as RawLoopNode}
      disabled={disabled}
      onChangeFields={onChangeFields}
    />
  );
}

function EnvironmentControl(props: LoopEditorFieldControlProps<FieldOfType<"environment">>) {
  return (
    <LoopEditorEnvironmentField
      disabled={props.disabled}
      field={props.field}
      gitBacked={props.gitBacked}
      lintIssues={props.lintIssues}
      loopDefaultEnvironment={props.loopDefaultEnvironment}
      onChange={props.onChange}
      onChangeFields={props.onChangeFields}
      raw={props.raw as RawLoopNode}
      worktrees={props.worktrees}
    />
  );
}

function RoutesControl({
  field,
  suggestions,
  disabled,
  onChangeFields,
}: LoopEditorFieldControlProps<FieldOfType<"routes">>) {
  return (
    <LoopEditorRoute
      disabled={disabled}
      onChangeFields={onChangeFields}
      spec={field}
      suggestions={suggestions}
    />
  );
}

function StrategyControl({
  field,
  disabled,
  onChangeFields,
}: LoopEditorFieldControlProps<FieldOfType<"strategy">>) {
  return (
    <LoopEditorStrategyField disabled={disabled} onChangeFields={onChangeFields} spec={field} />
  );
}

function ReviewControl({
  field,
  suggestions,
  disabled,
  onChangeFields,
}: LoopEditorFieldControlProps<FieldOfType<"review">>) {
  return (
    <LoopEditorReviewField
      disabled={disabled}
      onChangeFields={onChangeFields}
      spec={field}
      suggestions={suggestions}
    />
  );
}

function FoldControl(props: LoopEditorFieldControlProps<FieldOfType<"fold">>) {
  const { field } = props;
  return (
    <LoopEditorFold defaultOpen={field.defaultOpen} label={field.label} subLabel={field.subLabel}>
      {field.fields.map(child => (
        <LoopEditorField key={child.key} {...props} field={child} />
      ))}
    </LoopEditorFold>
  );
}

const FIELD_CONTROLS: FieldControls = {
  text: LoopEditorTextField,
  textarea: LoopEditorTextField,
  number: LoopEditorNumberField,
  select: LoopEditorSelectField,
  switch: LoopEditorSwitchField,
  static: LoopEditorStaticField,
  hint: LoopEditorHintField,
  criteria: LoopEditorCriteriaField,
  events: LoopEditorEventsField,
  effects: LoopEditorEffectsField,
  "wait-mode": WaitModeControl,
  environment: EnvironmentControl,
  routes: RoutesControl,
  strategy: StrategyControl,
  review: ReviewControl,
  fold: FoldControl,
};

/** Renders one inspector field from its DSL-derived descriptor. */
export function LoopEditorField(props: LoopEditorFieldProps) {
  // The table is keyed by `field.type`, so the looked-up control always receives its own variant.
  const Control = FIELD_CONTROLS[props.field.type] as ComponentType<LoopEditorFieldProps>;
  return <Control {...props} />;
}
