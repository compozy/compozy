import type { ComponentPropsWithoutRef } from "react";

import { Plus, Trash2 } from "lucide-react";

import { Button, cn } from "@agh/ui";

import { useLocalRowKeys } from "@/hooks/use-local-row-keys";

import type { RolesRuntimeOptions } from "../hooks/use-roles-runtime-options";
import type { RoleRuntimeValue } from "../lib/roles-config";
import { fallbackFieldId } from "../lib/roles-validation";
import type { RoleFallbackEntry, RoleName } from "../types";
import { RoleRuntimeSelector } from "./role-runtime-selector";

interface RoleFallbackEditorProps extends Omit<ComponentPropsWithoutRef<"div">, "role"> {
  role: RoleName;
  entries: readonly RoleFallbackEntry[];
  options: RolesRuntimeOptions;
  errors: Record<string, string>;
  disabled?: boolean;
  testId: string;
  onAdd: () => void;
  onRemove: (index: number) => void;
  onUpdate: (index: number, value: RoleRuntimeValue) => void;
  registerFieldRef: (id: string) => (element: HTMLElement | null) => void;
}

/**
 * Ordered fallback routes for one role. Each route is a single runtime
 * decision, so it is edited through one runtime selector rather than a row of
 * free-text fields.
 */
export function RoleFallbackEditor({
  role,
  entries,
  options,
  errors,
  disabled,
  testId,
  onAdd,
  onRemove,
  onUpdate,
  registerFieldRef,
  className,
  ...rootProps
}: RoleFallbackEditorProps) {
  // Stable client-side row keys so React keys editable rows without the array
  // index. Append/remove keep keys aligned; length mismatches (mount / external
  // load) are normalized by useLocalRowKeys without mutating refs during render.
  const rowKeys = useLocalRowKeys(entries, "fallback");

  const handleAddRoute = () => {
    rowKeys.append();
    onAdd();
  };
  const handleRemoveRoute = (index: number) => {
    rowKeys.remove(index);
    onRemove(index);
  };

  return (
    <div {...rootProps} className={cn("flex flex-col gap-3", className)} data-testid={testId}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-0.5">
          <span className="text-ws-name font-medium text-fg-strong">Fallback chain</span>
          <span className="text-form-label text-muted">
            Ordered routes tried before session acceptance when the primary fails.
          </span>
        </div>
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          onClick={handleAddRoute}
          data-testid={`${testId}-add`}
        >
          <Plus className="size-3" aria-hidden="true" />
          Add route
        </Button>
      </div>
      {entries.length === 0 ? (
        <p className="text-form-label text-subtle" data-testid={`${testId}-empty`}>
          No fallback routes configured.
        </p>
      ) : (
        <ul className="flex flex-col gap-2" data-testid={`${testId}-list`}>
          {entries.map((entry, index) => {
            const id = fallbackFieldId(role, index);
            const error = errors[id];
            return (
              <li
                key={rowKeys.keys[index]}
                className="flex flex-col gap-1"
                data-testid={`${testId}-entry-${index}`}
              >
                <div className="flex items-center gap-2">
                  <span aria-hidden="true" className="w-4 shrink-0 font-mono text-badge text-faint">
                    {index + 1}
                  </span>
                  {/* Register the selector's own trigger so a blocked save focuses
                      the control the operator has to fix, not a wrapper. */}
                  <div
                    ref={node => registerFieldRef(id)(node?.querySelector("button") ?? null)}
                    className="min-w-0 flex-1"
                  >
                    <RoleRuntimeSelector
                      value={entry}
                      options={options}
                      disabled={disabled}
                      triggerTestId={`${id}-select`}
                      onChange={value => onUpdate(index, value)}
                    />
                  </div>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    disabled={disabled}
                    aria-label={`Remove fallback route ${index + 1}`}
                    data-testid={`${testId}-remove-${index}`}
                    onClick={() => handleRemoveRoute(index)}
                  >
                    <Trash2 className="size-3" aria-hidden="true" />
                  </Button>
                </div>
                {error ? (
                  <span className="pl-6 text-form-hint text-danger" role="alert">
                    {error}
                  </span>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
