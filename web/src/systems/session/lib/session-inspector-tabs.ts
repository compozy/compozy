/**
 * The inspector's tab registry.
 *
 * Three things have to agree for a tab to actually exist: the id union, the
 * strip entry, and the testid every selector targets. Keeping them in one module
 * — rather than beside the component — means the registry can be checked for
 * gaps without mounting anything, and `satisfies` makes a half-registered tab a
 * typecheck failure rather than a tab that renders but cannot be found.
 */
export const SESSION_INSPECTOR_TABS = [
  { id: "usage", label: "Usage" },
  { id: "memory", label: "Memory" },
  { id: "files", label: "Files" },
  { id: "vault", label: "Vault" },
  { id: "calls", label: "Calls" },
] as const satisfies ReadonlyArray<{ id: string; label: string }>;

export type InspectorTabId = (typeof SESSION_INSPECTOR_TABS)[number]["id"];

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

type InspectorTabListener = (tab: InspectorTabId) => void;

const inspectorTabListeners = new Set<InspectorTabListener>();

/** Ask the open inspector to switch tab. Transcript rows use this for Calls. */
export function requestSessionInspectorTab(tab: InspectorTabId): void {
  for (const listener of inspectorTabListeners) {
    listener(tab);
  }
}

export function subscribeSessionInspectorTab(listener: InspectorTabListener): () => void {
  inspectorTabListeners.add(listener);
  return () => {
    inspectorTabListeners.delete(listener);
  };
}
