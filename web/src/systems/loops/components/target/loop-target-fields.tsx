import { useId } from "react";

import { Alert, AlertDescription, Button, Field, FieldLabel, Spinner } from "@compozy/ui";
import {
  networkParticipationDraftFromPayload,
  serializeNetworkParticipation,
} from "@/lib/network-participation";

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
import { loopInputCatalogNeeds } from "../../lib/loop-input-catalogs";
import { LoopInputCatalogBoundary } from "../input/loop-input-catalogs";
import { LoopCatalogValueSelect } from "../input/loop-typed-input-control";
import { LoopInputMapping } from "./loop-input-mapping";
import { NetworkParticipationFields } from "@/systems/network";

interface LoopTargetFieldsProps {
  catalog: LoopTargetCatalog;
  mode: "create" | "edit";
  value: LoopTargetDraft;
  onChange: (value: LoopTargetDraft) => void;
  identityDisabled?: boolean;
  /** Show the event-payload mapping table (triggers/webhooks only). */
  showMapping?: boolean;
  workspaceId?: string;
}

export function LoopTargetFields({
  catalog,
  mode,
  value,
  onChange,
  identityDisabled = false,
  showMapping = false,
  workspaceId = "",
}: LoopTargetFieldsProps) {
  const instanceId = useId();
  const loopControlId = `${instanceId}-loop-target`;
  const selected = catalog.selected;
  const inputs = selected?.inputs ?? {};
  const inputNames = Object.keys(inputs);
  const selectedIsUnavailable =
    value.loop_name !== "" &&
    (catalog.status === "incompatible" || catalog.status === "unavailable");
  const noticeId = `${instanceId}-loop-target-availability`;
  const compatibilityMessage = loopTargetAvailabilityMessage(catalog, mode);
  const hasSelectableContent = catalog.options.length > 0 || selectedIsUnavailable;

  return (
    <div className="space-y-4" data-testid="loop-target-fields">
      <Field>
        <FieldLabel htmlFor={loopControlId}>Loop</FieldLabel>
        {catalog.isLoading && catalog.options.length === 0 && !selected ? (
          <div className="flex h-9 items-center gap-2 text-form-hint text-subtle">
            <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
            Loading Loops…
          </div>
        ) : catalog.error && catalog.options.length === 0 && !selected ? (
          <LoopCatalogValueSelect
            describedBy={compatibilityMessage ? noticeId : undefined}
            catalog={{
              options: [],
              loading: false,
              error: catalog.error.message,
            }}
            controlId={loopControlId}
            disabled={identityDisabled}
            label="Loop"
            onChange={next => onChange(setLoopTargetLoop(value, next))}
            testId="loop-target-select"
            value={value.loop_name}
          />
        ) : !hasSelectableContent && catalog.hasNextPage ? (
          <p className="text-form-hint text-subtle">No compatible Loops loaded yet.</p>
        ) : !hasSelectableContent ? (
          <p className="text-form-hint text-subtle">
            No Loops in this workspace allow {catalog.requiredStartKind} starts.
          </p>
        ) : (
          <LoopCatalogValueSelect
            allowManual={false}
            describedBy={compatibilityMessage ? noticeId : undefined}
            catalog={{
              options: catalog.options.map(loop => ({ value: loop.name, label: loop.name })),
              loading: catalog.isLoading,
              error: catalog.error?.message ?? null,
            }}
            controlId={loopControlId}
            disabled={identityDisabled}
            label="Loop"
            onChange={next => onChange(setLoopTargetLoop(value, next))}
            testId="loop-target-select"
            value={value.loop_name}
          />
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
        <LoopInputCatalogBoundary workspaceId={workspaceId} needs={loopInputCatalogNeeds(inputs)}>
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
        </LoopInputCatalogBoundary>
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
