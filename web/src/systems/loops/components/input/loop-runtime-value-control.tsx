import { cn } from "@compozy/ui";

import {
  normalizeRuntimeACPSelections,
  REASONING_EFFORT_ORDER,
  RuntimeSelector,
  type RuntimeSelectorValue,
} from "@/systems/runtime";
import type { RuntimeSpeed } from "@/lib/api-contract";

import { useLoopInputCatalogs } from "../../hooks/use-loop-input-catalogs";

interface LoopRuntimeValueControlProps {
  value: unknown;
  controlId: string;
  testId: string;
  disabled?: boolean;
  invalid?: boolean;
  describedBy?: string;
  onChange: (value: unknown) => void;
}

function runtimeValue(value: unknown): RuntimeSelectorValue {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { provider: "", model: "", reasoning_effort: "" };
  }
  const record = value as Record<string, unknown>;
  const reasoning = typeof record.reasoning === "string" ? record.reasoning : "";
  const knownReasoning = new Set<string>(REASONING_EFFORT_ORDER);
  return {
    provider: typeof record.provider === "string" ? record.provider : "",
    model: typeof record.model === "string" ? record.model : "",
    reasoning_effort: knownReasoning.has(reasoning)
      ? (reasoning as RuntimeSelectorValue["reasoning_effort"])
      : "",
    acp_options: normalizeRuntimeACPSelections(
      Array.isArray(record.acp_options)
        ? (record.acp_options as Array<{
            id: string;
            value_id?: string;
            bool_value?: boolean;
          }>)
        : undefined
    ),
  };
}

function runtimeSpeed(value: unknown): RuntimeSpeed | undefined {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return undefined;
  const speed = (value as Record<string, unknown>).speed;
  return speed === "normal" || speed === "fast" ? speed : undefined;
}

function emittedRuntime(
  value: RuntimeSelectorValue,
  speed?: RuntimeSpeed
): Record<string, unknown> {
  const next: Record<string, unknown> = {};
  if (value.provider) next.provider = value.provider;
  if (value.model) next.model = value.model;
  if (value.reasoning_effort) next.reasoning = value.reasoning_effort;
  if (speed) next.speed = speed;
  if (value.acp_options) next.acp_options = value.acp_options;
  return next;
}

export function LoopRuntimeValueControl({
  value,
  controlId,
  testId,
  disabled,
  invalid,
  onChange,
}: LoopRuntimeValueControlProps) {
  const catalogs = useLoopInputCatalogs();
  const current = runtimeValue(value);
  const currentSpeed = runtimeSpeed(value);
  return (
    <RuntimeSelector
      allowCustomProvider
      ariaLabelledby={undefined}
      catalogStatus={catalogs.runtimeError}
      className={cn("w-full", invalid && "border-danger")}
      disabled={disabled}
      loading={catalogs.runtimeLoading}
      models={catalogs.runtimeModels}
      onChange={(next, normalizedSpeed) =>
        onChange(emittedRuntime(next, normalizedSpeed ?? currentSpeed))
      }
      onSpeedChange={speed => onChange(emittedRuntime(current, speed))}
      onRefreshCatalog={catalogs.refreshRuntime}
      providers={catalogs.runtimeProviders}
      refreshing={catalogs.refreshingRuntime}
      speed={currentSpeed ?? "normal"}
      triggerId={controlId}
      triggerTestId={testId}
      value={current}
    />
  );
}
