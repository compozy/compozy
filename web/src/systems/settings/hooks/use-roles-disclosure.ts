import { useState } from "react";

import { roleFromFieldId } from "../lib/roles-validation";
import type { RoleName } from "../types";

export interface RolesDisclosure {
  isOpen: (role: RoleName) => boolean;
  setOpen: (role: RoleName, open: boolean) => void;
  expandAll: () => void;
  collapseAll: () => void;
  /** True when at least one role body is open, so the bulk control can invert. */
  anyOpen: boolean;
}

export interface RolesDisclosureInput {
  roles: readonly { role: RoleName; hasDiagnostics: boolean }[];
  /** Field-id → message map; a role holding a blocking error is never collapsed. */
  validationErrors: Record<string, string>;
}

/**
 * Open state for the role accordion. Every role starts collapsed; a role is
 * forced open when the daemon reports a diagnostic for it or when it holds a
 * validation error, so nothing that blocks save can hide behind a closed row.
 */
export function useRolesDisclosure({
  roles,
  validationErrors,
}: RolesDisclosureInput): RolesDisclosure {
  const [userOpen, setUserOpen] = useState<Record<string, boolean>>({});

  const forcedOpen = new Set<string>();
  for (const role of roles) {
    if (role.hasDiagnostics) {
      forcedOpen.add(role.role);
    }
  }
  for (const id of Object.keys(validationErrors)) {
    forcedOpen.add(roleFromFieldId(id));
  }

  const isOpen = (role: RoleName) => forcedOpen.has(role) || Boolean(userOpen[role]);

  return {
    isOpen,
    setOpen: (role, open) => setUserOpen(current => ({ ...current, [role]: open })),
    expandAll: () =>
      setUserOpen(Object.fromEntries(roles.map(entry => [entry.role, true] as const))),
    collapseAll: () => setUserOpen({}),
    anyOpen: roles.some(entry => isOpen(entry.role)),
  };
}
