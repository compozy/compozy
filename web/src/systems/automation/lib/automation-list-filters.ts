import type { Filter, FilterFieldsConfig } from "@agh/ui";

import { automationScopeLabel, automationSourceLabel } from "./automation-formatters";
import type { AutomationKind, AutomationScopeFilter, AutomationSource } from "../types";

export interface AutomationFilterState {
  scope: AutomationScopeFilter;
  source: AutomationSource | null;
  enabled: boolean | null;
  event: string | null;
}

export interface AutomationFilterHandlers {
  onScopeChange: (next: AutomationScopeFilter | null) => void;
  onSourceChange: (next: AutomationSource | null) => void;
  onEnabledChange: (next: boolean | null) => void;
  onEventChange: (next: string | null) => void;
}

const SCOPE_OPTIONS: Exclude<AutomationScopeFilter, "all">[] = ["global", "workspace"];
const SOURCE_OPTIONS: AutomationSource[] = ["dynamic", "config", "package"];

export function buildAutomationFilterFields(kind: AutomationKind): FilterFieldsConfig<string> {
  const fields: FilterFieldsConfig<string> = [
    {
      key: "scope",
      label: "Scope",
      type: "select",
      options: SCOPE_OPTIONS.map(value => ({ value, label: automationScopeLabel(value) })),
    },
    {
      key: "source",
      label: "Source",
      type: "select",
      options: SOURCE_OPTIONS.map(value => ({ value, label: automationSourceLabel(value) })),
    },
    {
      key: "enabled",
      label: "Status",
      type: "select",
      options: [
        { value: "true", label: "Enabled" },
        { value: "false", label: "Disabled" },
      ],
    },
  ];

  if (kind === "triggers") {
    fields.push({ key: "event", label: "Event", type: "text", placeholder: "ext.github.push" });
  }

  return fields;
}

export function automationFiltersToChips(state: AutomationFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.scope !== "all") {
    chips.push({
      id: "automation-filter-scope",
      field: "scope",
      operator: "is",
      values: [state.scope],
    });
  }
  if (state.source) {
    chips.push({
      id: "automation-filter-source",
      field: "source",
      operator: "is",
      values: [state.source],
    });
  }
  if (state.enabled !== null) {
    chips.push({
      id: "automation-filter-enabled",
      field: "enabled",
      operator: "is",
      values: [state.enabled ? "true" : "false"],
    });
  }
  if (state.event) {
    chips.push({
      id: "automation-filter-event",
      field: "event",
      operator: "is",
      values: [state.event],
    });
  }
  return chips;
}

export function applyAutomationFilterChips(
  chips: Filter<string>[],
  handlers: AutomationFilterHandlers
): void {
  const lookup = new Map<string, string | undefined>();
  for (const chip of chips) {
    lookup.set(chip.field, chip.values[0]);
  }
  handlers.onScopeChange(asAutomationScope(lookup.get("scope")));
  handlers.onSourceChange(asAutomationSource(lookup.get("source")));
  handlers.onEnabledChange(asAutomationEnabled(lookup.get("enabled")));
  handlers.onEventChange(lookup.get("event")?.trim() || null);
}

export function parseAutomationScope(value: unknown): AutomationScopeFilter | undefined {
  return value === "all" || value === "global" || value === "workspace" ? value : undefined;
}

export function parseAutomationSource(value: unknown): AutomationSource | undefined {
  return value === "config" || value === "package" || value === "dynamic" ? value : undefined;
}

export function parseAutomationEnabled(value: unknown): boolean | undefined {
  if (value === true || value === "true") return true;
  if (value === false || value === "false") return false;
  return undefined;
}

function asAutomationScope(value: string | undefined): AutomationScopeFilter | null {
  return value === "global" || value === "workspace" ? value : null;
}

function asAutomationSource(value: string | undefined): AutomationSource | null {
  return value === "config" || value === "package" || value === "dynamic" ? value : null;
}

function asAutomationEnabled(value: string | undefined): boolean | null {
  if (value === "true") return true;
  if (value === "false") return false;
  return null;
}
