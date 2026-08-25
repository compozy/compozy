/**
 * The inspector's tab registry.
 *
 * Three things have to agree for a tab to actually exist: the id union, the
 * strip entry, and the testid every selector targets. Keeping them in one module
 * — rather than beside the component — means the registry can be checked for
 * gaps without mounting anything, and `satisfies` makes a half-registered tab a
 * typecheck failure rather than a tab that renders but cannot be found.
 */
export type InspectorTabId = "usage" | "memory" | "files" | "vault" | "calls";

export const SESSION_INSPECTOR_TABS = [
  { id: "usage", label: "Usage" },
  { id: "memory", label: "Memory" },
  { id: "files", label: "Files" },
  { id: "vault", label: "Vault" },
  { id: "calls", label: "Calls" },
] as const satisfies ReadonlyArray<{ id: InspectorTabId; label: string }>;

export const SESSION_INSPECTOR_TAB_TESTIDS = {
  usage: "session-inspector-tab-usage",
  memory: "session-inspector-tab-memory",
  files: "session-inspector-tab-files",
  vault: "session-inspector-tab-vault",
  calls: "session-inspector-tab-calls",
} as const satisfies Record<InspectorTabId, string>;

export const SESSION_INSPECTOR_TAB_IDS: readonly InspectorTabId[] = SESSION_INSPECTOR_TABS.map(
  tab => tab.id
);

export function isInspectorTabId(value: string): value is InspectorTabId {
  return SESSION_INSPECTOR_TAB_IDS.includes(value as InspectorTabId);
}
