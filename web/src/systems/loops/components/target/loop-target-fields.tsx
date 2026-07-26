import {
  Alert,
  AlertDescription,
  Button,
  Field,
  FieldLabel,
  NativeSelect,
  NativeSelectOption,
  Spinner,
} from "@agh/ui";
import {
  NetworkParticipationFields,
  networkParticipationDraftFromPayload,
  serializeNetworkParticipation,
} from "@/systems/network";

import {
  setLoopTargetInput,
  setLoopTargetLoop,
  setLoopTargetMapping,
  setLoopTargetNetworkParticipation,
  type LoopTargetDraft,
} from "../../lib/loop-target";
import {
  loopTargetAvailabilityMessage,
  type LoopTargetCatalog,
} from "../../lib/loop-target-availability";
import { LoopInputControl } from "./loop-input-control";
import { LoopInputMapping } from "./loop-input-mapping";

interface LoopTargetFieldsProps {
  catalog: LoopTargetCatalog;
  mode: "create" | "edit";
  value: LoopTargetDraft;
  onChange: (value: LoopTargetDraft) => void;
  identityDisabled?: boolean;
  /** Show the event-payload mapping table (triggers/webhooks only). */
  showMapping?: boolean;
}

/**
 * The Run-loop target editor (TechSpec §9.14): a loop picker, an auto-generated
 * typed-input form from the chosen Loop's declared inputs, and — for
 * triggers/webhooks — an event-payload -> input mapping table. Rendered from the
 * daemon's declared-input schema, so it always matches what the Loop accepts.
 */
export function LoopTargetFields({
  catalog,
  mode,
  value,
  onChange,
  identityDisabled = false,
  showMapping = false,
}: LoopTargetFieldsProps) {
  const selected = catalog.selected;
  const inputs = selected?.inputs ?? {};
  const inputNames = Object.keys(inputs);
  const selectedIsUnavailable =
    value.loop_name !== "" &&
    (catalog.status === "incompatible" || catalog.status === "unavailable");
  const noticeId = "loop-target-availability";
  const compatibilityMessage = loopTargetAvailabilityMessage(catalog, mode);
  const hasSelectableContent = catalog.options.length > 0 || selectedIsUnavailable;

  return (
    <div className="space-y-4" data-testid="loop-target-fields">
      <Field>
        <FieldLabel htmlFor="loop-target-loop">Loop</FieldLabel>
        {catalog.isLoading && catalog.options.length === 0 && !selected ? (
          <div className="flex h-9 items-center gap-2 text-form-hint text-subtle">
            <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
            Loading Loops…
          </div>
        ) : catalog.error && catalog.options.length === 0 && !selected ? (
          <p className="text-form-hint text-danger" role="alert">
            Could not load Loops for this workspace.
          </p>
        ) : !hasSelectableContent && catalog.hasNextPage ? (
          <p className="text-form-hint text-subtle">No compatible Loops loaded yet.</p>
        ) : !hasSelectableContent ? (
          <p className="text-form-hint text-subtle">
            No Loops in this workspace allow {catalog.requiredStartKind} starts.
          </p>
        ) : (
          <NativeSelect
            aria-describedby={compatibilityMessage ? noticeId : undefined}
            id="loop-target-loop"
            data-testid="loop-target-select"
            disabled={identityDisabled}
            value={value.loop_name}
            onChange={event => onChange(setLoopTargetLoop(value, event.target.value))}
          >
            <NativeSelectOption value="">Select a loop</NativeSelectOption>
            {selectedIsUnavailable ? (
              <NativeSelectOption disabled value={value.loop_name}>
                {value.loop_name} (unavailable for {catalog.requiredStartKind})
              </NativeSelectOption>
            ) : null}
            {catalog.options.map(loop => (
              <NativeSelectOption key={loop.name} value={loop.name}>
                {loop.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        )}
        {compatibilityMessage ? (
          <Alert id={noticeId} role="alert" variant="warning">
            <AlertDescription>{compatibilityMessage}</AlertDescription>
          </Alert>
        ) : null}
        {catalog.error && catalog.options.length > 0 ? (
          <p className="text-form-hint text-danger" role="alert">
            {catalog.error.message}
          </p>
        ) : null}
        {catalog.hasNextPage ? (
          <Button
            aria-busy={catalog.isFetchingNextPage}
            data-testid="loop-target-load-more"
            disabled={catalog.isFetchingNextPage}
            onClick={catalog.fetchNextPage}
            size="sm"
            type="button"
            variant="ghost"
          >
            {catalog.isFetchingNextPage ? "Loading more Loops…" : "Load more Loops"}
          </Button>
        ) : null}
      </Field>

      <NetworkParticipationFields
        allowedStrategies={["named", "loop_run"]}
        onChange={next =>
          onChange(setLoopTargetNetworkParticipation(value, serializeNetworkParticipation(next)))
        }
        testIdPrefix="loop-target-participation"
        value={networkParticipationDraftFromPayload(value.network_participation)}
      />

      {selected && inputNames.length > 0 ? (
        <div className="space-y-3" data-testid="loop-target-inputs">
          {inputNames.map(name => (
            <LoopInputControl
              key={name}
              name={name}
              field={inputs[name]}
              value={value.inputs?.[name]}
              onChange={next => onChange(setLoopTargetInput(value, name, next))}
            />
          ))}
        </div>
      ) : selected ? (
        <p className="text-form-hint text-subtle">This Loop declares no inputs.</p>
      ) : null}

      {selected && showMapping ? (
        <LoopInputMapping
          inputs={inputs}
          mapping={value.input_mapping ?? {}}
          onChange={(key, path) => onChange(setLoopTargetMapping(value, key, path))}
        />
      ) : null}
    </div>
  );
}
