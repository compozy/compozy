import { useKeyedLocalStoragePreference } from "@/hooks/use-keyed-local-storage-preference";

export type InspectorTab = "members" | "work";

interface InspectorChannelState {
  open: boolean;
  tab: InspectorTab;
}

const STORAGE_KEY = "network:inspector-state";
const DEFAULT_STATE: InspectorChannelState = { open: false, tab: "members" };

function decodeInspectorState(value: unknown): InspectorChannelState | null {
  if (value === null || typeof value !== "object") return null;
  const { open, tab } = value as { open?: unknown; tab?: unknown };
  if (typeof open !== "boolean") return null;
  return { open, tab: tab === "members" || tab === "work" ? tab : DEFAULT_STATE.tab };
}

export interface UseInspectorStateResult {
  open: boolean;
  tab: InspectorTab;
  toggle: () => void;
  close: () => void;
  openWith: (tab: InspectorTab) => void;
  setTab: (tab: InspectorTab) => void;
}

/**
 * Per-channel inspector state — open/closed + active sub-tab — persisted in
 * localStorage per `_design.md` §5.8.3 + the user's "Inspector default closed,
 * remembered per channel" decision.
 */
export function useInspectorState(channel: string | null | undefined): UseInspectorStateResult {
  const key = channel ?? "";
  const [state, persist] = useKeyedLocalStoragePreference({
    storageKey: STORAGE_KEY,
    itemKey: key,
    defaultValue: DEFAULT_STATE,
    decode: decodeInspectorState,
  });

  const toggle = () => {
    persist({ open: !state.open, tab: state.tab });
  };

  const close = () => {
    persist({ open: false, tab: state.tab });
  };

  const openWith = (tab: InspectorTab) => {
    persist({ open: true, tab });
  };

  const setTab = (tab: InspectorTab) => {
    persist({ open: state.open, tab });
  };

  return { open: state.open, tab: state.tab, toggle, close, openWith, setTab };
}
