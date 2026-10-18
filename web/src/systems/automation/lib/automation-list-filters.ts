import type { Filter, FilterFieldsConfig } from "@compozy/ui";

import { automationSourceLabel } from "./automation-formatters";
import type { AutomationDoes } from "./automation-sentence";
import type { AutomationScope, AutomationSource } from "../types";

/** One value per facet, operator "is" (Business Rule 4). */
export interface AutomationFilterState {
  target: AutomationDoes | null;
  enabled: boolean | null;
  scope: AutomationScope | null;
  source: AutomationSource | null;
  loop: string | null;
}

export interface AutomationFilterHandlers {
  onTargetChange: (next: AutomationDoes | null) => void;
  onEnabledChange: (next: boolean | null) => void;
  onScopeChange: (next: AutomationScope | null) => void;
  onSourceChange: (next: AutomationSource | null) => void;
  onLoopChange: (next: string | null) => void;
}

export const AUTOMATION_DOES_LABELS = {
  agent: "Ask an agent",
  loop: "Start a Loop",
  task: "Create a task",
} as const satisfies Record<AutomationDoes, string>;

const SOURCE_OPTIONS: AutomationSource[] = ["dynamic", "config", "package"];

/** Facets: Does · Status · Location · Source · Loop. Start views replace the old Event filter. */
export function buildAutomationFilterFields(): FilterFieldsConfig<string> {
  return [
    {
      key: "target",
      label: "Does",
      type: "select",
      options: (Object.keys(AUTOMATION_DOES_LABELS) as AutomationDoes[]).map(value => ({
        value,
        label: AUTOMATION_DOES_LABELS[value],
      })),
    },
    {
      key: "enabled",
      label: "Status",
      type: "select",
      options: [
        { value: "true", label: "On" },
        { value: "false", label: "Off" },
      ],
    },
    {
      key: "scope",
      label: "Location",
      type: "select",
      options: [
        { value: "workspace", label: "This project" },
        { value: "global", label: "Global" },
      ],
    },
    {
      key: "source",
      label: "Source",
      type: "select",
      options: SOURCE_OPTIONS.map(value => ({ value, label: automationSourceLabel(value) })),
    },
    { key: "loop", label: "Loop", type: "text", placeholder: "software-delivery" },
  ];
}

function chip(field: keyof AutomationFilterState, value: string): Filter<string> {
  return { id: `automation-filter-${field}`, field, operator: "is", values: [value] };
}

export function automationFiltersToChips(state: AutomationFilterState): Filter<string>[] {
  const chips: Filter<string>[] = [];
  if (state.target) chips.push(chip("target", state.target));
  if (state.enabled !== null) chips.push(chip("enabled", state.enabled ? "true" : "false"));
  if (state.scope) chips.push(chip("scope", state.scope));
  if (state.source) chips.push(chip("source", state.source));
  if (state.loop) chips.push(chip("loop", state.loop));
  return chips;
}

export function applyAutomationFilterChips(
  chips: Filter<string>[],
  handlers: AutomationFilterHandlers
): void {
  const lookup = new Map<string, string | undefined>();
  for (const entry of chips) {
    lookup.set(entry.field, entry.values[0]);
  }
  handlers.onTargetChange(asTarget(lookup.get("target")));
  handlers.onEnabledChange(asEnabled(lookup.get("enabled")));
  handlers.onScopeChange(asScope(lookup.get("scope")));
  handlers.onSourceChange(asSource(lookup.get("source")));
  handlers.onLoopChange(lookup.get("loop")?.trim() || null);
}

function asTarget(value: string | undefined): AutomationDoes | null {
  return value === "agent" || value === "loop" || value === "task" ? value : null;
}

function asScope(value: string | undefined): AutomationScope | null {
  return value === "global" || value === "workspace" ? value : null;
}

function asSource(value: string | undefined): AutomationSource | null {
  return value === "config" || value === "package" || value === "dynamic" ? value : null;
}

function asEnabled(value: string | undefined): boolean | null {
  if (value === "true") return true;
  if (value === "false") return false;
  return null;
}
