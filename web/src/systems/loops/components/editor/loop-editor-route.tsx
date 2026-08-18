import { useEffect, useRef } from "react";
import { ChevronDown, ChevronUp, Plus, X } from "lucide-react";

import {
  Button,
  cn,
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  Input,
  Label,
  NativeSelect,
  NativeSelectOption,
  RequiredMark,
} from "@compozy/ui";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { NodeFieldEdit } from "../../lib/loop-editor-draft";
import { routeDefaultEdit, routesEdit } from "../../lib/loop-node-route-fields";
import type { LoopRouteEntry, RoutesFieldSpec } from "../../lib/loop-node-schema-types";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { LoopReferenceInput } from "./loop-reference-input";

export interface LoopEditorRouteProps {
  spec: RoutesFieldSpec;
  disabled?: boolean;
  onChangeFields: (edits: NodeFieldEdit[]) => void;
  suggestions: readonly LoopReferenceSuggestion[];
}

const EMPTY_ROUTE: LoopRouteEntry = { when: "", to: "" };
const CONTROL_CLASS = "h-8 px-2.5 font-mono text-form-input";
const ADD_ROUTE_FOCUS = '[data-focus-key="add"]';

function moveEntry(routes: readonly LoopRouteEntry[], index: number, delta: number) {
  const target = index + delta;
  if (target < 0 || target >= routes.length) return [...routes];
  const next = [...routes];
  const [entry] = next.splice(index, 1);
  next.splice(target, 0, entry);
  return next;
}

function moveFocusSelector(index: number, delta: number, total: number) {
  const target = index + delta;
  const stillMovable = delta < 0 ? target > 0 : target < total - 1;
  const direction = stillMovable ? delta : -delta;
  return `[data-focus-key="${direction < 0 ? "up" : "down"}-${target}"]`;
}

function removeFocusSelector(index: number, total: number) {
  const remaining = total - 1;
  if (remaining === 0) return ADD_ROUTE_FOCUS;
  return `[data-focus-key="remove-${Math.min(index, remaining - 1)}"]`;
}

function addFocusSelector(index: number) {
  return `[data-testid="loop-route-when-${index}"]`;
}

export function LoopEditorRoute({
  spec,
  disabled = false,
  onChangeFields,
  suggestions,
}: LoopEditorRouteProps) {
  const rowKeys = useLocalRowKeys(spec.routes, "route");

  const focus = useRef<{ root: HTMLDivElement | null; pending: string | null }>({
    root: null,
    pending: null,
  });

  useEffect(() => {
    const pending = focus.current.pending;
    if (pending === null) return;
    focus.current.pending = null;
    focus.current.root?.querySelector<HTMLElement>(pending)?.focus();
  });

  function commit(routes: readonly LoopRouteEntry[], focusSelector: string | null) {
    focus.current.pending = focusSelector;
    onChangeFields(routesEdit(routes));
  }

  return (
    <>
      <Field data-slot="loop-node-routes-field">
        <FieldHeader>
          <FieldLabel>{spec.label}</FieldLabel>
        </FieldHeader>
        <div
          className="flex flex-col gap-2"
          data-testid="loop-editor-routes"
          ref={element => {
            focus.current.root = element;
          }}
        >
          {spec.routes.length === 0 ? (
            <p className="text-form-hint leading-relaxed text-subtle">
              No routes declared. Every run takes the default.
            </p>
          ) : null}
          {spec.routes.map((route, index) => (
            <RouteRow
              key={rowKeys.keys[index]}
              disabled={disabled}
              index={index}
              route={route}
              suggestions={suggestions}
              targets={spec.targets}
              total={spec.routes.length}
              onChange={next =>
                commit(
                  spec.routes.map((entry, position) => (position === index ? next : entry)),
                  null
                )
              }
              onMove={delta =>
                commit(
                  moveEntry(spec.routes, index, delta),
                  moveFocusSelector(index, delta, spec.routes.length)
                )
              }
              onRemove={() => {
                rowKeys.remove(index);
                commit(
                  spec.routes.filter((_, position) => position !== index),
                  removeFocusSelector(index, spec.routes.length)
                );
              }}
            />
          ))}
          <Button
            className="border-dashed"
            data-focus-key="add"
            data-testid="loop-route-add"
            disabled={disabled}
            onClick={() => {
              rowKeys.append();
              commit([...spec.routes, EMPTY_ROUTE], addFocusSelector(spec.routes.length));
            }}
            size="sm"
            type="button"
            variant="outline"
          >
            <Plus aria-hidden="true" className="size-3" />
            Add route
          </Button>
        </div>
        {spec.hint ? <FieldDescription>{spec.hint}</FieldDescription> : null}
      </Field>
      <Field data-slot="loop-node-route-default-field">
        <FieldHeader>
          <FieldLabel>default</FieldLabel>
          <RequiredMark>*</RequiredMark>
        </FieldHeader>
        <TargetSelect
          ariaLabel="Default route"
          disabled={disabled}
          onChange={next => onChangeFields(routeDefaultEdit(next))}
          targets={spec.targets}
          testId="loop-route-default"
          value={spec.defaultRoute}
        />
        <FieldDescription>Where a run goes when no route matches.</FieldDescription>
      </Field>
    </>
  );
}

interface RouteRowProps {
  route: LoopRouteEntry;
  index: number;
  total: number;
  targets: readonly string[];
  suggestions: readonly LoopReferenceSuggestion[];
  disabled: boolean;
  onChange: (next: LoopRouteEntry) => void;
  onMove: (delta: number) => void;
  onRemove: () => void;
}

function RouteRow({
  route,
  index,
  total,
  targets,
  suggestions,
  disabled,
  onChange,
  onMove,
  onRemove,
}: RouteRowProps) {
  const position = index + 1;
  return (
    <div
      className="rounded-md border border-line-soft bg-canvas-soft p-2.5"
      data-testid={`loop-route-row-${index}`}
    >
      <div className="mb-2 flex items-center gap-2">
        <span className="font-mono text-mono-id text-fg-strong">#{position}</span>
        <div className="ml-auto flex items-center gap-0.5">
          <Button
            aria-label={`Move route ${position} up`}
            data-focus-key={`up-${index}`}
            disabled={disabled || index === 0}
            onClick={() => onMove(-1)}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <ChevronUp aria-hidden="true" className="size-3" />
          </Button>
          <Button
            aria-label={`Move route ${position} down`}
            data-focus-key={`down-${index}`}
            disabled={disabled || index === total - 1}
            onClick={() => onMove(1)}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <ChevronDown aria-hidden="true" className="size-3" />
          </Button>
          <Button
            aria-label={`Remove route ${position}`}
            data-focus-key={`remove-${index}`}
            data-testid={`loop-route-remove-${index}`}
            disabled={disabled}
            onClick={onRemove}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <X aria-hidden="true" className="size-3" />
          </Button>
        </div>
      </div>
      <div className="flex flex-col gap-2">
        <Label className="text-badge text-faint">
          when
          <CelConditionInput
            ariaLabel={`Route ${position} when`}
            className="mt-1"
            disabled={disabled}
            onChange={next => onChange({ ...route, when: next })}
            placeholder={'severity == "p0"'}
            suggestions={suggestions}
            testId={`loop-route-when-${index}`}
            value={route.when}
          />
        </Label>
        <Label className="text-badge text-faint">
          to
          <TargetSelect
            ariaLabel={`Route ${position} to`}
            className="mt-1"
            disabled={disabled}
            onChange={next => onChange({ ...route, to: next })}
            targets={targets}
            testId={`loop-route-to-${index}`}
            value={route.to}
          />
        </Label>
      </div>
    </div>
  );
}

interface CelConditionInputProps {
  ariaLabel: string;
  className?: string;
  disabled: boolean;
  onChange: (next: string) => void;
  placeholder: string;
  suggestions: readonly LoopReferenceSuggestion[];
  testId: string;
  value: string;
}

function CelConditionInput({
  ariaLabel,
  className,
  disabled,
  onChange,
  placeholder,
  suggestions,
  testId,
  value,
}: CelConditionInputProps) {
  if (suggestions.length === 0) {
    return (
      <Input
        aria-label={ariaLabel}
        className={cn(CONTROL_CLASS, className)}
        data-testid={testId}
        disabled={disabled}
        onChange={event => onChange(event.target.value)}
        placeholder={placeholder}
        type="text"
        value={value}
      />
    );
  }
  return (
    <div className={className}>
      <LoopReferenceInput
        ariaLabel={ariaLabel}
        cel
        disabled={disabled}
        mono
        onChange={onChange}
        placeholder={placeholder}
        suggestions={suggestions}
        testId={testId}
        value={value}
      />
    </div>
  );
}

interface TargetSelectProps {
  ariaLabel: string;
  className?: string;
  disabled: boolean;
  onChange: (next: string) => void;
  targets: readonly string[];
  testId: string;
  value: string;
}

function TargetSelect({
  ariaLabel,
  className,
  disabled,
  onChange,
  targets,
  testId,
  value,
}: TargetSelectProps) {
  const stale = value !== "" && !targets.includes(value);
  return (
    <NativeSelect
      aria-label={ariaLabel}
      className={className}
      data-testid={testId}
      disabled={disabled}
      onChange={event => onChange(event.target.value)}
      value={value}
    >
      <NativeSelectOption value="">
        {targets.length === 0 ? "No forward nodes" : "Select a target"}
      </NativeSelectOption>
      {stale ? <NativeSelectOption value={value}>{value} (not forward)</NativeSelectOption> : null}
      {targets.map(target => (
        <NativeSelectOption key={target} value={target}>
          {target}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  );
}
