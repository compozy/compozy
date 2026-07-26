import { useId, useState } from "react";

import { Collapsible, CollapsibleContent } from "@agh/ui";

import type { RolesRuntimeOptions } from "../hooks/use-roles-runtime-options";
import type { RoleRuntimeValue } from "../lib/roles-config";
import { roleFieldId } from "../lib/roles-validation";
import type { RoleViewModel } from "../lib/roles-view-model";
import type { RoleName } from "../types";
import { RoleAdvancedDetails } from "./role-advanced-details";
import { RoleDiagnosticsNotice } from "./role-diagnostics-notice";
import { RoleFieldControl } from "./role-field-control";
import { RolePanelHeader } from "./role-panel-header";
import { RoleRoutingFields } from "./role-routing-fields";

const TEST_PREFIX = "settings-page-roles";

export interface RolePanelProps {
  vm: RoleViewModel;
  options: RolesRuntimeOptions;
  validationErrors: Record<string, string>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  disabled?: boolean;
  draftRevision?: number;
  setRoleEnabled: (role: RoleName, enabled: boolean) => void;
  setRoleAgent: (role: RoleName, agent: string) => void;
  setRoleField: (role: RoleName, field: string, value: string | number | boolean) => void;
  setRoleRuntime: (role: RoleName, value: RoleRuntimeValue) => void;
  clearRuntime: (role: RoleName) => void;
  setNumberFieldValidity: (id: string) => (message: string | null) => void;
  addFallback: (role: RoleName) => void;
  removeFallback: (role: RoleName, index: number) => void;
  updateFallback: (role: RoleName, index: number, value: RoleRuntimeValue) => void;
  registerFieldRef: (id: string) => (element: HTMLElement | null) => void;
}

/**
 * One role as a disclosure row: the collapsed header answers "what is this,
 * how does it route, is it on"; expanding reveals the two routing decisions,
 * this role's policy fields, and an Advanced fold for the operator layer.
 */
export function RolePanel({
  vm,
  options,
  validationErrors,
  open,
  onOpenChange,
  disabled,
  draftRevision,
  setRoleEnabled,
  setRoleAgent,
  setRoleField,
  setRoleRuntime,
  clearRuntime,
  setNumberFieldValidity,
  addFallback,
  removeFallback,
  updateFallback,
  registerFieldRef,
}: RolePanelProps) {
  const { role, fields, advancedFields, values, effective, status } = vm;
  const bodyId = `role-panel-${useId().replace(/:/g, "")}`;
  const testId = `${TEST_PREFIX}-${role}`;
  const [advancedOpen, setAdvancedOpen] = useState(false);
  // A blocking error inside the fold force-opens it, so the save flow can focus
  // the offending field instead of aiming at a hidden control.
  const hasAdvancedError = Object.keys(validationErrors).some(
    id =>
      id.startsWith(`${role}.fallback.`) ||
      advancedFields.some(field => id === roleFieldId(role, field.key))
  );

  return (
    <Collapsible
      open={open}
      onOpenChange={onOpenChange}
      className="not-last:border-b not-last:border-line-soft"
      data-testid={`${TEST_PREFIX}-group-${role}`}
    >
      <RolePanelHeader
        vm={vm}
        bodyId={bodyId}
        disabled={disabled}
        testId={testId}
        onEnabledChange={enabled => setRoleEnabled(role, enabled)}
      />
      <CollapsibleContent id={bodyId} keepMounted>
        <div className="flex flex-col gap-4 px-4 pt-1 pb-4 pl-10">
          <RoleDiagnosticsNotice
            diagnostics={status.diagnostics}
            data-testid={`${testId}-diagnostics`}
          />
          {/* One recessed well, not a stack of frames: routing and policy rows
              read as a single list separated by hairlines. */}
          <div className="overflow-hidden rounded-lg border border-line-soft bg-canvas">
            <RoleRoutingFields
              vm={vm}
              options={options}
              disabled={disabled}
              testId={testId}
              onAgentChange={agent => setRoleAgent(role, agent)}
              onRuntimeChange={value => setRoleRuntime(role, value)}
              onClearRuntime={() => clearRuntime(role)}
            />
            {fields.map(field => {
              const id = roleFieldId(role, field.key);
              return (
                <RoleFieldControl
                  key={field.key}
                  field={field}
                  value={values[field.key]}
                  hasEffective={field.key in effective}
                  effective={field.key in effective ? effective[field.key] : null}
                  error={validationErrors[id]}
                  disabled={disabled}
                  testId={`${testId}-${field.key}`}
                  resetRevision={draftRevision}
                  fieldRef={field.kind === "number" ? registerFieldRef(id) : undefined}
                  onValueChange={value => setRoleField(role, field.key, value)}
                  onValidityChange={
                    field.kind === "number" ? setNumberFieldValidity(id) : undefined
                  }
                />
              );
            })}
          </div>
          <RoleAdvancedDetails
            vm={vm}
            options={options}
            errors={validationErrors}
            open={advancedOpen || hasAdvancedError}
            onOpenChange={setAdvancedOpen}
            disabled={disabled}
            draftRevision={draftRevision}
            testId={`${testId}-advanced`}
            setRoleField={setRoleField}
            setNumberFieldValidity={setNumberFieldValidity}
            onAdd={() => addFallback(role)}
            onRemove={index => removeFallback(role, index)}
            onUpdate={(index, value) => updateFallback(role, index, value)}
            registerFieldRef={registerFieldRef}
          />
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}
