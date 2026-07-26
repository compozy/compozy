import { Plus, X } from "lucide-react";

import { Button, Label, NativeSelect, NativeSelectOption } from "@agh/ui";
import { LOOP_WATCH_EVENT_KINDS } from "@/generated/loop-enums";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { MonoTag } from "../mono-tag";
import { LoopReferenceInput } from "./loop-reference-input";

type Subscription = Record<string, unknown> & { kind?: string; filter?: string };

interface LoopEditorWatchEventsProps {
  value: unknown;
  onChange: (subscriptions: Subscription[]) => void;
  suggestions?: readonly LoopReferenceSuggestion[];
  disabled?: boolean;
}

// The default kind for a freshly added row — the first supported phase-A kind. The full
// option list is the GENERATED matrix (`LOOP_WATCH_EVENT_KINDS`, sourced from the Go
// SupportedWatchEvents registry) so the select never hand-authors the enum.
const DEFAULT_WATCH_EVENT_KIND = LOOP_WATCH_EVENT_KINDS[0];
const EMPTY_REFERENCE_SUGGESTIONS: readonly LoopReferenceSuggestion[] = [];

function asSubscriptions(value: unknown): Subscription[] {
  return Array.isArray(value)
    ? (value.filter(item => item && typeof item === "object") as Subscription[])
    : [];
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/**
 * The watch-events subscription list editor (WS1): one row per subscription with a kind
 * select fed by the generated supported-kind matrix and a CEL filter over `event` /
 * `inputs` / `nodes`. Editing writes the whole `events` array back through the codec
 * verbatim; the shared linter owns kind/filter validation on publish.
 */
export function LoopEditorWatchEvents({
  value,
  onChange,
  suggestions = EMPTY_REFERENCE_SUGGESTIONS,
  disabled = false,
}: LoopEditorWatchEventsProps) {
  const subscriptions = asSubscriptions(value);
  // Row IDs are component-local: they stabilize editor instances without entering the loop DSL.
  const rowKeys = useLocalRowKeys(subscriptions, "watch-event");

  const update = (index: number, patch: Record<string, unknown>) => {
    onChange(subscriptions.map((entry, i) => (i === index ? { ...entry, ...patch } : entry)));
  };
  const remove = (index: number) => {
    rowKeys.remove(index);
    onChange(subscriptions.filter((_, i) => i !== index));
  };
  const add = () => {
    rowKeys.append();
    onChange([...subscriptions, { kind: DEFAULT_WATCH_EVENT_KIND }]);
  };

  return (
    <div className="flex flex-col gap-2" data-testid="loop-editor-watch-events">
      {subscriptions.map((subscription, index) => (
        <div
          key={rowKeys.keys[index]}
          className="rounded-md border border-line-soft bg-canvas-soft p-2.5"
        >
          <div className="mb-2 flex items-center gap-2">
            <MonoTag className="rounded-xs bg-badge-fill px-1.5 py-0.5 text-pill-group-badge text-subtle">
              event
            </MonoTag>
            <span className="font-mono text-mono-id text-fg-strong">#{index + 1}</span>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              disabled={disabled}
              onClick={() => remove(index)}
              aria-label={`Remove subscription ${index + 1}`}
              className="ml-auto"
            >
              <X aria-hidden="true" className="size-3" />
            </Button>
          </div>
          <div className="flex flex-col gap-2">
            <Label className="text-badge text-faint">
              kind
              <NativeSelect
                className="mt-1"
                value={str(subscription.kind) || DEFAULT_WATCH_EVENT_KIND}
                disabled={disabled}
                onChange={event => update(index, { kind: event.target.value })}
                aria-label={`Subscription ${index + 1} kind`}
                data-testid={`loop-watch-event-kind-${index}`}
              >
                {LOOP_WATCH_EVENT_KINDS.map(option => (
                  <NativeSelectOption key={option} value={option}>
                    {option}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Label>
            <Label className="text-badge text-faint">
              filter
              <div className="mt-1">
                <LoopReferenceInput
                  value={str(subscription.filter)}
                  onChange={next => update(index, { filter: next })}
                  suggestions={suggestions}
                  multiline
                  mono
                  cel
                  placeholder="event.payload.to_status == 'completed'"
                  ariaLabel={`Subscription ${index + 1} filter`}
                  testId={`loop-watch-event-filter-${index}`}
                />
              </div>
            </Label>
          </div>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={add}
        className="border-dashed"
        data-testid="loop-editor-watch-events-add"
      >
        <Plus aria-hidden="true" className="size-3" />
        Add subscription
      </Button>
    </div>
  );
}
