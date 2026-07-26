import type { RolesRuntimeOptions } from "../hooks/use-roles-runtime-options";
import type { RoleRuntimeValue } from "../lib/roles-config";
import { roleFieldId } from "../lib/roles-validation";
import type { RoleViewModel } from "../lib/roles-view-model";
import type { RoleName } from "../types";
import { RoleFallbackEditor } from "./role-fallback-editor";
import { RoleFieldControl } from "./role-field-control";
import { SettingsAdvancedFold, SettingsProvChip } from "./settings-advanced-fold";

export interface RoleAdvancedDetailsProps {
  vm: RoleViewModel;
  options: RolesRuntimeOptions;
  errors: Record<string, string>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  disabled?: boolean;
  draftRevision?: number;
  testId: string;
  setRoleField: (role: RoleName, field: string, value: string | number | boolean) => void;
  setNumberFieldValidity: (id: string) => (message: string | null) => void;
  onAdd: () => void;
  onRemove: (index: number) => void;
  onUpdate: (index: number, value: RoleRuntimeValue) => void;
  registerFieldRef: (id: string) => (element: HTMLElement | null) => void;
}

/**
 * The operator layer for one role: tuning knobs, the ordered fallback chain,
 * and the config provenance the daemon returned. Rendered bare so it does not
 * frame itself inside the role panel that already frames the row.
 */
export function RoleAdvancedDetails({
  vm,
  options,
  errors,
  open,
  onOpenChange,
  disabled,
  draftRevision,
  testId,
  setRoleField,
  setNumberFieldValidity,
  onAdd,
  onRemove,
  onUpdate,
  registerFieldRef,
}: RoleAdvancedDetailsProps) {
  const { role, advancedFields, values, status } = vm;
  const provenanceKeys = Object.keys(status.provenance);

  return (
    <SettingsAdvancedFold
      bare
      open={open}
      onOpenChange={onOpenChange}
      label="Advanced — fallback routes"
      data-testid={testId}
    >
      <div className="flex flex-col gap-6 pt-4">
        {advancedFields.length > 0 ? (
          <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
            {advancedFields.map(field => {
              const id = roleFieldId(role, field.key);
              return (
                <RoleFieldControl
                  key={field.key}
                  field={field}
                  value={values[field.key]}
                  hasEffective={false}
                  effective={null}
                  error={errors[id]}
                  disabled={disabled}
                  testId={`settings-page-roles-${role}-${field.key}`}
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
        ) : null}
        <RoleFallbackEditor
          role={role}
          entries={vm.fallbackChain}
          options={options}
          errors={errors}
          disabled={disabled}
          testId={`${testId}-fallback`}
          onAdd={onAdd}
          onRemove={onRemove}
          onUpdate={onUpdate}
          registerFieldRef={registerFieldRef}
        />
        {provenanceKeys.length > 0 ? (
          <div className="flex flex-col gap-1.5" data-testid={`${testId}-provenance`}>
            <span className="text-form-label font-medium text-muted">Config provenance</span>
            <div className="flex flex-wrap items-center gap-1.5">
              {provenanceKeys.map(key => (
                <SettingsProvChip key={key}>
                  {key} · {status.provenance[key]}
                </SettingsProvChip>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </SettingsAdvancedFold>
  );
}
