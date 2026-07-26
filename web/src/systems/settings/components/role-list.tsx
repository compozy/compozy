import { Button } from "@agh/ui";

import type { RolesDisclosure } from "../hooks/use-roles-disclosure";
import type { RolesRuntimeOptions } from "../hooks/use-roles-runtime-options";
import type { RoleRuntimeValue } from "../lib/roles-config";
import type { RoleViewModel } from "../lib/roles-view-model";
import type { RoleName } from "../types";
import { RolePanel } from "./role-panel";

const TEST_PREFIX = "settings-page-roles";

export interface RoleListProps {
  roles: readonly RoleViewModel[];
  options: RolesRuntimeOptions;
  disclosure: RolesDisclosure;
  validationErrors: Record<string, string>;
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

function summarize(roles: readonly RoleViewModel[]): string {
  const off = roles.filter(role => !role.enabled).length;
  const roleCount = `${roles.length} ${roles.length === 1 ? "role" : "roles"}`;
  return off > 0 ? `${roleCount} · ${off} off` : roleCount;
}

/** The six background roles as one list of disclosure rows. */
export function RoleList({ roles, disclosure, ...panelProps }: RoleListProps) {
  const allExpanded = roles.every(role => disclosure.isOpen(role.role));

  return (
    <section className="flex flex-col gap-2.5">
      <header className="flex items-center justify-between gap-3">
        <span className="text-form-label text-muted" data-testid={`${TEST_PREFIX}-count`}>
          {summarize(roles)}
        </span>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          data-testid={`${TEST_PREFIX}-expand-toggle`}
          onClick={allExpanded ? disclosure.collapseAll : disclosure.expandAll}
        >
          {allExpanded ? "Collapse all" : "Expand all"}
        </Button>
      </header>
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
        {roles.map(vm => (
          <RolePanel
            key={vm.role}
            vm={vm}
            open={disclosure.isOpen(vm.role)}
            onOpenChange={open => disclosure.setOpen(vm.role, open)}
            {...panelProps}
          />
        ))}
      </div>
    </section>
  );
}
