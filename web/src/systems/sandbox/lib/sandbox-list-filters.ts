import type { Filter, FilterFieldsConfig } from "@agh/ui";

import {
  SANDBOX_BACKENDS,
  SANDBOX_PERSISTENCE_OPTIONS,
  type SandboxBackend,
  type SandboxPersistence,
} from "./sandbox-labels";

export type SandboxBackendFilter = SandboxBackend | "all";
export type SandboxPersistenceFilter = SandboxPersistence | "all";

export interface SandboxFilterState {
  backend: SandboxBackendFilter;
  persistence: SandboxPersistenceFilter;
}

export interface SandboxFilterHandlers {
  onBackendChange: (next: SandboxBackendFilter) => void;
  onPersistenceChange: (next: SandboxPersistenceFilter) => void;
}

export function buildSandboxFilterFields(): FilterFieldsConfig<string> {
  return [
    {
      key: "backend",
      label: "Backend",
      type: "select",
      options: SANDBOX_BACKENDS.map(value => ({ value, label: value })),
    },
    {
      key: "persistence",
      label: "Persistence",
      type: "select",
      options: SANDBOX_PERSISTENCE_OPTIONS.map(value => ({ value, label: value })),
    },
  ];
}

export function sandboxFiltersToChips(state: SandboxFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.backend !== "all") {
    chips.push({
      id: "sandbox-filter-backend",
      field: "backend",
      operator: "is",
      values: [state.backend],
    });
  }
  if (state.persistence !== "all") {
    chips.push({
      id: "sandbox-filter-persistence",
      field: "persistence",
      operator: "is",
      values: [state.persistence],
    });
  }
  return chips;
}

export function applySandboxFilterChips(
  chips: Filter<string>[],
  handlers: SandboxFilterHandlers
): void {
  const backend = chips.find(chip => chip.field === "backend")?.values[0];
  const persistence = chips.find(chip => chip.field === "persistence")?.values[0];
  handlers.onBackendChange(asSandboxBackend(backend));
  handlers.onPersistenceChange(asSandboxPersistence(persistence));
}

export function parseSandboxBackendFilter(value: unknown): SandboxBackend | undefined {
  if (typeof value !== "string") return undefined;
  return (SANDBOX_BACKENDS as readonly string[]).includes(value)
    ? (value as SandboxBackend)
    : undefined;
}

export function parseSandboxPersistenceFilter(value: unknown): SandboxPersistence | undefined {
  if (typeof value !== "string") return undefined;
  return (SANDBOX_PERSISTENCE_OPTIONS as readonly string[]).includes(value)
    ? (value as SandboxPersistence)
    : undefined;
}

function asSandboxBackend(value: string | undefined): SandboxBackendFilter {
  if (!value) return "all";
  return (SANDBOX_BACKENDS as readonly string[]).includes(value)
    ? (value as SandboxBackend)
    : "all";
}

function asSandboxPersistence(value: string | undefined): SandboxPersistenceFilter {
  if (!value) return "all";
  return (SANDBOX_PERSISTENCE_OPTIONS as readonly string[]).includes(value)
    ? (value as SandboxPersistence)
    : "all";
}
