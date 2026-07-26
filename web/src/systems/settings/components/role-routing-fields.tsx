import { useId } from "react";

import { AgentCommandSelect } from "@/systems/agent";
import { Button } from "@agh/ui";

import type { RolesRuntimeOptions } from "../hooks/use-roles-runtime-options";
import type { RoleRuntimeValue } from "../lib/roles-config";
import type { RoleViewModel } from "../lib/roles-view-model";
import { RoleEffectiveHint } from "./role-effective-hint";
import { RoleRuntimeSelector } from "./role-runtime-selector";
import { SettingsFieldRow } from "./settings-field-row";

export interface RoleRoutingFieldsProps {
  vm: RoleViewModel;
  options: RolesRuntimeOptions;
  disabled?: boolean;
  testId: string;
  onAgentChange: (agent: string) => void;
  onRuntimeChange: (value: RoleRuntimeValue) => void;
  onClearRuntime: () => void;
}

/**
 * The two decisions that define a role's route: which agent it runs as, and
 * which runtime that agent uses. Both read their effective value from the
 * daemon projection rather than restating the draft.
 */
export function RoleRoutingFields({
  vm,
  options,
  disabled,
  testId,
  onAgentChange,
  onRuntimeChange,
  onClearRuntime,
}: RoleRoutingFieldsProps) {
  const runtimeLabelId = `${useId().replace(/:/g, "")}-runtime-label`;
  const noProviders = !options.loading && options.providers.length === 0;

  return (
    <>
      {vm.supportsAgent ? (
        <SettingsFieldRow
          data-testid={`${testId}-agent`}
          label="Agent"
          description={
            <>
              <span>Route to a catalog agent, or keep the role default.</span>
              <RoleEffectiveHint
                effective={vm.effective.agent ?? null}
                emptyLabel="Resolves at invocation."
              />
            </>
          }
          control={
            <AgentCommandSelect
              agents={options.agents}
              value={vm.agent || null}
              clearLabel="Role default"
              disabled={disabled}
              className="w-64"
              triggerTestId={`${testId}-agent-select`}
              onChange={next => onAgentChange(next ?? "")}
            />
          }
        />
      ) : null}
      <SettingsFieldRow
        data-testid={`${testId}-runtime`}
        label={<span id={runtimeLabelId}>Runtime</span>}
        description={
          <>
            <span>Provider, model and reasoning effort for this role.</span>
            {noProviders ? (
              <span className="mt-0.5 block text-form-hint text-warning">
                No providers configured yet.
              </span>
            ) : (
              <RoleEffectiveHint effective={vm.routeSummary} emptyLabel="Resolves at invocation." />
            )}
          </>
        }
        control={
          <div className="flex items-center gap-2">
            {vm.hasRuntimeOverride ? (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                disabled={disabled}
                data-testid={`${testId}-runtime-clear`}
                onClick={onClearRuntime}
              >
                Clear override
              </Button>
            ) : null}
            <RoleRuntimeSelector
              value={vm.runtime}
              options={options}
              disabled={disabled}
              ariaLabelledby={runtimeLabelId}
              triggerTestId={`${testId}-runtime-select`}
              onChange={onRuntimeChange}
            />
          </div>
        }
      />
    </>
  );
}
