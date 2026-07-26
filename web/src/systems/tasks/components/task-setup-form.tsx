import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Textarea,
} from "@agh/ui";

import {
  RuntimeSelector,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";

import type { TaskExecutionProfileSetRequest } from "../types";

export interface TaskSetupFormProps {
  value: TaskExecutionProfileSetRequest;
  onChange: (value: TaskExecutionProfileSetRequest) => void;
  runtime: {
    providers: RuntimeProviderOption[];
    models: RuntimeModelOption[];
    providersLoading: boolean;
    catalogLoading: boolean;
    catalogLoaded: boolean;
    catalogRefreshing: boolean;
    errorMessage?: string | null;
    onRefreshCatalog: () => void;
  };
}

function listValue(values?: string[]): string {
  return values?.join(", ") ?? "";
}

function parseList(value: string): string[] | undefined {
  const items = value
    .split(",")
    .map(item => item.trim())
    .filter(Boolean);
  return items.length > 0 ? items : undefined;
}

function runtimeValue(provider?: string, model?: string): RuntimeSelectorValue {
  return { provider: provider ?? "", model: model ?? "", reasoning_effort: "" };
}

export function TaskSetupForm({ value, onChange, runtime }: TaskSetupFormProps) {
  const update = <Key extends keyof TaskExecutionProfileSetRequest>(
    key: Key,
    next: TaskExecutionProfileSetRequest[Key]
  ) => onChange({ ...value, [key]: next });

  return (
    <FieldGroup className="gap-5" data-testid="tasks-setup-form">
      <fieldset className="flex flex-col gap-3">
        <legend className="eyebrow mb-1 text-subtle">Worker</legend>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="tasks-setup-worker-mode">Selection mode</FieldLabel>
            <NativeSelect
              id="tasks-setup-worker-mode"
              onChange={event =>
                update("worker", {
                  ...value.worker,
                  mode: event.target.value as TaskExecutionProfileSetRequest["worker"]["mode"],
                })
              }
              value={value.worker.mode}
            >
              <NativeSelectOption value="inherit">Inherit workspace</NativeSelectOption>
              <NativeSelectOption value="select">Select by policy</NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldLabel htmlFor="tasks-setup-worker-agent">Preferred agent</FieldLabel>
            <Input
              id="tasks-setup-worker-agent"
              onChange={event =>
                update("worker", {
                  ...value.worker,
                  agent_name: event.target.value.trim() || undefined,
                })
              }
              value={value.worker.agent_name ?? ""}
            />
          </Field>
        </div>
        <Field>
          <FieldLabel id="tasks-setup-worker-runtime-label">Runtime</FieldLabel>
          <RuntimeSelector
            ariaLabelledby="tasks-setup-worker-runtime-label"
            catalogLoaded={runtime.catalogLoaded}
            loading={runtime.catalogLoading}
            models={runtime.models}
            onChange={next =>
              update("worker", {
                ...value.worker,
                provider: next.provider || undefined,
                model: next.model || undefined,
              })
            }
            onRefreshCatalog={runtime.onRefreshCatalog}
            providers={runtime.providers}
            refreshing={runtime.catalogRefreshing}
            triggerId="tasks-setup-worker-runtime"
            triggerTestId="tasks-setup-worker-runtime"
            value={runtimeValue(value.worker.provider, value.worker.model)}
          />
          {runtime.errorMessage ? (
            <p className="text-form-hint text-warning">{runtime.errorMessage}</p>
          ) : null}
        </Field>
        <Field>
          <FieldLabel htmlFor="tasks-setup-worker-allowed">Allowed agents</FieldLabel>
          <FieldDescription>Comma-separated agent names.</FieldDescription>
          <Input
            defaultValue={listValue(value.worker.allowed_agent_names)}
            id="tasks-setup-worker-allowed"
            onBlur={event =>
              update("worker", {
                ...value.worker,
                allowed_agent_names: parseList(event.target.value),
              })
            }
          />
        </Field>
      </fieldset>

      <fieldset className="flex flex-col gap-3 border-t border-line-soft pt-5">
        <legend className="eyebrow mb-1 text-subtle">Coordinator</legend>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field>
            <FieldLabel htmlFor="tasks-setup-coordinator-mode">Mode</FieldLabel>
            <NativeSelect
              id="tasks-setup-coordinator-mode"
              onChange={event =>
                update("coordinator", {
                  ...value.coordinator,
                  mode: event.target.value as TaskExecutionProfileSetRequest["coordinator"]["mode"],
                })
              }
              value={value.coordinator.mode}
            >
              <NativeSelectOption value="inherit">Inherit workspace</NativeSelectOption>
              <NativeSelectOption value="guided">Guided</NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldLabel htmlFor="tasks-setup-coordinator-agent">Agent</FieldLabel>
            <Input
              id="tasks-setup-coordinator-agent"
              onChange={event =>
                update("coordinator", {
                  ...value.coordinator,
                  agent_name: event.target.value.trim() || undefined,
                })
              }
              value={value.coordinator.agent_name ?? ""}
            />
          </Field>
        </div>
        <Field>
          <FieldLabel htmlFor="tasks-setup-coordinator-guidance">Guidance</FieldLabel>
          <Textarea
            id="tasks-setup-coordinator-guidance"
            onChange={event =>
              update("coordinator", {
                ...value.coordinator,
                guidance: event.target.value.trim() || undefined,
              })
            }
            rows={3}
            value={value.coordinator.guidance ?? ""}
          />
        </Field>
      </fieldset>

      <fieldset className="flex flex-col gap-3 border-t border-line-soft pt-5">
        <legend className="eyebrow mb-1 text-subtle">Review</legend>
        <Field>
          <FieldLabel id="tasks-setup-review-runtime-label">Reviewer runtime</FieldLabel>
          <RuntimeSelector
            ariaLabelledby="tasks-setup-review-runtime-label"
            catalogLoaded={runtime.catalogLoaded}
            loading={runtime.catalogLoading}
            models={runtime.models}
            onChange={next =>
              update("review", {
                ...value.review,
                provider: next.provider || undefined,
                model: next.model || undefined,
              })
            }
            onRefreshCatalog={runtime.onRefreshCatalog}
            providers={runtime.providers}
            refreshing={runtime.catalogRefreshing}
            triggerId="tasks-setup-review-runtime"
            triggerTestId="tasks-setup-review-runtime"
            value={runtimeValue(value.review.provider, value.review.model)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="tasks-setup-review-agents">Allowed reviewers</FieldLabel>
          <FieldDescription>Comma-separated agent names.</FieldDescription>
          <Input
            defaultValue={listValue(value.review.allowed_agent_names)}
            id="tasks-setup-review-agents"
            onBlur={event =>
              update("review", {
                ...value.review,
                allowed_agent_names: parseList(event.target.value),
              })
            }
          />
        </Field>
      </fieldset>

      <fieldset className="grid gap-3 border-t border-line-soft pt-5 sm:grid-cols-2">
        <legend className="eyebrow mb-1 text-subtle">Sandbox and participants</legend>
        <Field>
          <FieldLabel htmlFor="tasks-setup-sandbox-mode">Sandbox</FieldLabel>
          <NativeSelect
            id="tasks-setup-sandbox-mode"
            onChange={event =>
              update("sandbox", {
                ...value.sandbox,
                mode: event.target.value as TaskExecutionProfileSetRequest["sandbox"]["mode"],
              })
            }
            value={value.sandbox.mode}
          >
            <NativeSelectOption value="inherit">Inherit workspace</NativeSelectOption>
            <NativeSelectOption value="none">None</NativeSelectOption>
            <NativeSelectOption value="ref">Named sandbox</NativeSelectOption>
          </NativeSelect>
        </Field>
        <Field>
          <FieldLabel htmlFor="tasks-setup-sandbox-ref">Sandbox reference</FieldLabel>
          <Input
            disabled={value.sandbox.mode !== "ref"}
            id="tasks-setup-sandbox-ref"
            onChange={event =>
              update("sandbox", {
                ...value.sandbox,
                sandbox_ref: event.target.value.trim() || undefined,
              })
            }
            value={value.sandbox.sandbox_ref ?? ""}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="tasks-setup-participants">Participant agents</FieldLabel>
          <Input
            defaultValue={listValue(value.participants.allowed_agent_names)}
            id="tasks-setup-participants"
            onBlur={event =>
              update("participants", {
                ...value.participants,
                allowed_agent_names: parseList(event.target.value),
              })
            }
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="tasks-setup-capabilities">Required capabilities</FieldLabel>
          <Input
            defaultValue={listValue(value.participants.required_capabilities)}
            id="tasks-setup-capabilities"
            onBlur={event =>
              update("participants", {
                ...value.participants,
                required_capabilities: parseList(event.target.value),
              })
            }
          />
        </Field>
      </fieldset>
    </FieldGroup>
  );
}
