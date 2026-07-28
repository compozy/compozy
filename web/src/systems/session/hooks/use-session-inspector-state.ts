import { useKeyedLocalStoragePreference } from "@/hooks/use-keyed-local-storage-preference";

const STORAGE_KEY = "session:inspector-open";
const DEFAULT_OPEN = false;

function decodeOpen(value: unknown): boolean | null {
  return typeof value === "boolean" ? value : null;
}

export interface UseSessionInspectorStateResult {
  open: boolean;
  toggle: () => void;
  close: () => void;
  setOpen: (open: boolean) => void;
}

/**
 * Per-session inspector open/closed preference — default closed, remembered in
 * localStorage so reopening a session restores the last rail visibility.
 */
export function useSessionInspectorState(
  sessionId: string | null | undefined
): UseSessionInspectorStateResult {
  const key = sessionId ?? "";
  const [open, persist] = useKeyedLocalStoragePreference({
    storageKey: STORAGE_KEY,
    itemKey: key,
    defaultValue: DEFAULT_OPEN,
    decode: decodeOpen,
  });

  return {
    open,
    toggle: () => {
      persist(!open);
    },
    close: () => {
      persist(false);
    },
    setOpen: (next: boolean) => {
      persist(next);
    },
  };
}
