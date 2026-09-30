import {
  Field,
  FieldDescription,
  FieldLabel,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Textarea,
} from "@compozy/ui";

import { AgentCommandSelect } from "@/systems/agent";
import { RuntimeSelector } from "@/systems/runtime";

import {
  SESSION_CONTINUE_DEFAULT_ROUTE,
  type SessionContinueDialogModel,
} from "../hooks/use-session-continue-dialog";
import type { SessionContinueRuntimeOptions } from "../hooks/use-session-continue-runtime-options";

const DEFAULT_ROUTE_LABEL = "Default";

export interface SessionContinueFieldsProps {
  model: SessionContinueDialogModel;
  runtimeOptions: SessionContinueRuntimeOptions;
}

/** Agent · Runtime XOR Route · First message — the Continue dialog's choices. */
export function SessionContinueFields({ model, runtimeOptions }: SessionContinueFieldsProps) {
  const disabled = model.isSubmitting;
  const routeSelected = model.route !== SESSION_CONTINUE_DEFAULT_ROUTE;
  const routeLabel =
    model.routes.find(option => option.route === model.route)?.label ?? DEFAULT_ROUTE_LABEL;

  return (
    <>
      <Field>
        <FieldLabel htmlFor="session-continue-agent">Agent</FieldLabel>
        <AgentCommandSelect
          agents={model.agents}
          disabled={disabled}
          loading={model.agentsLoading}
          onChange={next => {
            if (next) model.selectAgent(next);
          }}
          triggerId="session-continue-agent"
          triggerTestId="session-continue-agent-select"
          value={model.agentName || null}
        />
      </Field>

      {routeSelected ? null : (
        <Field>
          <FieldLabel id="session-continue-runtime-label">Runtime</FieldLabel>
          <RuntimeSelector
            ariaLabelledby="session-continue-runtime-label"
            disabled={disabled || runtimeOptions.providers.length === 0}
            loading={runtimeOptions.loading}
            models={runtimeOptions.models}
            onChange={model.onRuntimeChange}
            onRefreshCatalog={runtimeOptions.refresh}
            onSpeedChange={model.onRuntimeSpeedChange}
            providers={runtimeOptions.providers}
            refreshing={runtimeOptions.refreshing}
            speed={model.runtimeSpeed}
            triggerTestId="session-continue-runtime-select"
            value={model.runtimeValue}
            {...(runtimeOptions.catalogError ? { catalogStatus: runtimeOptions.catalogError } : {})}
          />
        </Field>
      )}

      {model.routes.length > 0 ? (
        <Field>
          <FieldLabel htmlFor="session-continue-route">Route</FieldLabel>
          <Select
            disabled={disabled}
            onValueChange={next => {
              if (typeof next === "string" && next !== "") model.onRouteChange(Number(next));
            }}
            value={String(model.route)}
          >
            <SelectTrigger
              className="w-full"
              data-testid="session-continue-route-select"
              id="session-continue-route"
            >
              <SelectValue>{routeLabel}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={String(SESSION_CONTINUE_DEFAULT_ROUTE)}>
                {DEFAULT_ROUTE_LABEL}
              </SelectItem>
              {model.routes.map(option => (
                <SelectItem key={option.route} value={String(option.route)}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FieldDescription>
            Default uses the agent's own provider, model and account. A route is one of the agent's
            declared fallback routes.
          </FieldDescription>
        </Field>
      ) : null}

      <Field>
        <FieldLabel htmlFor="session-continue-message">First message</FieldLabel>
        <Textarea
          data-testid="session-continue-message"
          disabled={disabled}
          id="session-continue-message"
          onChange={event => model.onMessageChange(event.target.value)}
          placeholder="Optional. Sent after the carried context."
          rows={3}
          value={model.message}
        />
      </Field>
    </>
  );
}
