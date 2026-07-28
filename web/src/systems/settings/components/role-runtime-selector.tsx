import { RuntimeSelector, type RuntimeSelectorValue } from "@/systems/runtime";

import type { RoleRuntimeValue } from "../lib/roles-config";
import type { RolesRuntimeOptions } from "../hooks/use-roles-runtime-options";

export interface RoleRuntimeSelectorProps {
  value: RoleRuntimeValue;
  options: RolesRuntimeOptions;
  disabled?: boolean;
  triggerId?: string;
  triggerTestId: string;
  ariaLabelledby?: string;
  onChange: (next: RoleRuntimeValue) => void;
}

/**
 * The role routing control: one runtime selector standing in for provider,
 * model and reasoning effort. Empty strings mean "inherit", exactly as the
 * `[roles]` section stores them.
 */
export function RoleRuntimeSelector({
  value,
  options,
  disabled,
  triggerId,
  triggerTestId,
  ariaLabelledby,
  onChange,
}: RoleRuntimeSelectorProps) {
  const selectorValue: RuntimeSelectorValue = {
    provider: value.provider,
    model: value.model,
    reasoning_effort: value.reasoning_effort as RuntimeSelectorValue["reasoning_effort"],
  };
  return (
    <RuntimeSelector
      variant="small"
      value={selectorValue}
      providers={options.providers}
      models={options.models}
      loading={options.loading}
      catalogLoaded={options.catalogLoaded}
      refreshing={options.refreshing}
      onRefreshCatalog={options.refresh}
      disabled={disabled}
      modelPlaceholder="Inherit"
      triggerId={triggerId}
      triggerTestId={triggerTestId}
      {...(ariaLabelledby ? { ariaLabelledby } : {})}
      {...(options.catalogError ? { catalogStatus: options.catalogError } : {})}
      onChange={next =>
        onChange({
          provider: next.provider,
          model: next.model,
          reasoning_effort: next.reasoning_effort,
        })
      }
    />
  );
}
