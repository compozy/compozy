import { useRef, useState, type SetStateAction } from "react";

import {
  SettingsApiError,
  useRolesStatus,
  useSettingsRoles,
  useUpdateSettingsRoles,
} from "@/systems/settings";

import {
  addFallbackEntry,
  applyRoleFieldEdit,
  applyRoleRuntimeEdit,
  clearRoleRuntime,
  removeFallbackEntry,
  ROLE_ORDER,
  setFallbackRuntime,
  type RoleRuntimeValue,
} from "../lib/roles-config";
import { buildRolesViewModel, type RoleViewModel } from "../lib/roles-view-model";
import {
  collectRoleValidationErrors,
  firstRoleFieldError,
  toRoleErrorMap,
} from "../lib/roles-validation";
import type { RoleName, SettingsRolesConfig, SettingsUpdateRolesRequest } from "../types";
import { useRolesDisclosure, type RolesDisclosure } from "./use-roles-disclosure";
import { useRolesRuntimeOptions, type RolesRuntimeOptions } from "./use-roles-runtime-options";
import { useSettingsPage } from "./use-settings-page";

type NumberErrors = Record<string, string | null>;

function filterActiveErrors(errors: NumberErrors): Record<string, string> {
  const active: Record<string, string> = {};
  for (const [id, message] of Object.entries(errors)) {
    if (message) {
      active[id] = message;
    }
  }
  return active;
}

function errorMessage(error: unknown): string | null {
  if (error instanceof SettingsApiError) {
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return null;
}

function hasCompleteRoleRoster(roles: readonly { role: string }[]): boolean {
  const received = new Set(roles.map(role => role.role));
  return roles.length === ROLE_ORDER.length && ROLE_ORDER.every(role => received.has(role));
}

/**
 * Roles Settings page view-model. Two reads gate the editor — the effective
 * projection (`useRolesStatus`) and the editable section (`useSettingsRoles`).
 * `handleRetry` refetches both. Save submits the full section through the
 * config-apply plane; validation covers every fallback entry and, on a save
 * attempt with an invalid entry, the draft is retained and the first invalid
 * field is focused instead of submitting.
 */
export function useSettingsRolesPage() {
  const statusQuery = useRolesStatus();
  const configQuery = useSettingsRoles();
  const mutation = useUpdateSettingsRoles();
  const page = useSettingsPage({ currentSlug: "roles" });

  const status = statusQuery.data ?? null;
  const envelope = configQuery.data ?? null;

  const [draftOverride, setDraftOverride] = useState<SettingsRolesConfig | null>();
  const [lastAppliedLabel, setLastAppliedLabel] = useState<string | null>(null);
  const [numberErrors, setNumberErrors] = useState<NumberErrors>({});
  const [draftRevision, setDraftRevision] = useState(0);
  const fieldRefs = useRef(new Map<string, HTMLElement | null>());

  const draft = draftOverride === undefined ? (envelope?.config ?? null) : draftOverride;
  const setDraft = (update: SetStateAction<SettingsRolesConfig | null>) => {
    setDraftOverride(current => {
      const resolved = current === undefined ? (envelope?.config ?? null) : current;
      return typeof update === "function" ? update(resolved) : update;
    });
  };

  const isDirty =
    envelope && draft ? JSON.stringify(envelope.config) !== JSON.stringify(draft) : false;

  const fallbackErrors = draft ? collectRoleValidationErrors(draft) : [];
  const validationErrors = {
    ...toRoleErrorMap(fallbackErrors),
    ...filterActiveErrors(numberErrors),
  };
  const isInvalid = Object.keys(validationErrors).length > 0;

  const projectionUnavailable = Boolean(status) && !hasCompleteRoleRoster(status?.roles ?? []);
  const roles: RoleViewModel[] =
    status && draft && !projectionUnavailable ? buildRolesViewModel(status.roles, draft) : [];

  const runtimeOptions = useRolesRuntimeOptions();
  const disclosure = useRolesDisclosure({ roles, validationErrors });

  const registerFieldRef = (id: string) => (element: HTMLElement | null) => {
    if (element) {
      fieldRefs.current.set(id, element);
    } else {
      fieldRefs.current.delete(id);
    }
  };

  // Defer focus to the next microtask so the invalid field is mounted (the
  // advanced fold opens from derived validationErrors) without a setState-in-effect.
  const focusField = (id: string | null) => {
    if (!id) {
      return;
    }
    queueMicrotask(() => {
      fieldRefs.current.get(id)?.focus();
    });
  };

  const setRoleField = (role: RoleName, field: string, value: string | number | boolean) =>
    setDraft(prev => (prev ? applyRoleFieldEdit(prev, role, field, value) : prev));
  const setRoleEnabled = (role: RoleName, enabled: boolean) =>
    setRoleField(role, "enabled", enabled);
  const setRoleAgent = (role: RoleName, agent: string) => setRoleField(role, "agent", agent);
  const setRoleRuntime = (role: RoleName, value: RoleRuntimeValue) =>
    setDraft(prev => (prev ? applyRoleRuntimeEdit(prev, role, value) : prev));
  const clearRuntime = (role: RoleName) =>
    setDraft(prev => (prev ? clearRoleRuntime(prev, role) : prev));
  const addFallback = (role: RoleName) =>
    setDraft(prev => (prev ? addFallbackEntry(prev, role) : prev));
  const removeFallback = (role: RoleName, index: number) =>
    setDraft(prev => (prev ? removeFallbackEntry(prev, role, index) : prev));
  const updateFallback = (role: RoleName, index: number, value: RoleRuntimeValue) =>
    setDraft(prev => (prev ? setFallbackRuntime(prev, role, index, value) : prev));

  const setNumberFieldValidity = (id: string) => (message: string | null) =>
    setNumberErrors(prev => (prev[id] === message ? prev : { ...prev, [id]: message }));

  const handleReset = () => {
    if (envelope) {
      setDraft(envelope.config);
    }
    setNumberErrors({});
    setDraftRevision(current => current + 1);
  };

  const handleSave = () => {
    if (!draft) {
      return;
    }
    const errors = collectRoleValidationErrors(draft);
    const combined = { ...toRoleErrorMap(errors), ...filterActiveErrors(numberErrors) };
    if (Object.keys(combined).length > 0) {
      focusField(firstRoleFieldError(errors)?.id ?? Object.keys(combined)[0] ?? null);
      return;
    }
    const body: SettingsUpdateRolesRequest = { config: draft };
    mutation.mutate(body, {
      onSuccess: () => setLastAppliedLabel("Saved · applied immediately"),
    });
  };

  const handleRetry = () => {
    void statusQuery.refetch();
    void configQuery.refetch();
  };

  return {
    isLoading: statusQuery.isLoading || configQuery.isLoading,
    isEmpty: projectionUnavailable,
    error: (statusQuery.error ?? configQuery.error) as Error | null,
    roles,
    runtimeOptions,
    disclosure,
    isDirty,
    isInvalid,
    draftRevision,
    validationErrors,
    isSaving: mutation.isPending,
    saveError: errorMessage(mutation.error),
    warnings: mutation.data?.warnings,
    lastAppliedLabel,
    restart: page.restart,
    setRoleEnabled,
    setRoleAgent,
    setRoleField,
    setRoleRuntime,
    clearRuntime,
    addFallback,
    removeFallback,
    updateFallback,
    setNumberFieldValidity,
    registerFieldRef,
    handleSave,
    handleReset,
    handleRetry,
  };
}

export type SettingsRolesPage = ReturnType<typeof useSettingsRolesPage>;
export type { RolesDisclosure, RolesRuntimeOptions };
