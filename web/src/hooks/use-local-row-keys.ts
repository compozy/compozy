import { useId, useState } from "react";

interface LocalRowKeyState {
  keys: string[];
  next: number;
}

function keysForLength(
  state: LocalRowKeyState,
  length: number,
  prefix: string,
  label: string
): LocalRowKeyState {
  if (state.keys.length === length) return state;
  if (state.keys.length > length) {
    return { ...state, keys: state.keys.slice(0, length) };
  }
  const added = Array.from(
    { length: length - state.keys.length },
    (_, index) => `${prefix}-${label}-${state.next + index}`
  );
  return {
    keys: [...state.keys, ...added],
    next: state.next + added.length,
  };
}

export function useLocalRowKeys(rows: readonly unknown[], label: string) {
  const prefix = useId();
  const [state, setState] = useState<LocalRowKeyState>({ keys: [], next: 0 });
  const current = keysForLength(state, rows.length, prefix, label);

  return {
    keys: current.keys,
    append: () => {
      setState(previous => {
        const normalized = keysForLength(previous, rows.length, prefix, label);
        return keysForLength(normalized, rows.length + 1, prefix, label);
      });
    },
    remove: (index: number) => {
      setState(previous => {
        const normalized = keysForLength(previous, rows.length, prefix, label);
        return {
          ...normalized,
          keys: normalized.keys.filter((_, keyIndex) => keyIndex !== index),
        };
      });
    },
  };
}
