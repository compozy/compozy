import { useState } from "react";
import type { Dispatch, SetStateAction } from "react";

const cachedState = new Map<string, unknown>();

export function useCachedState<T>(key: string, initialValue: T): [T, Dispatch<SetStateAction<T>>] {
  const [value, setLocalValue] = useState<T>(
    () => (cachedState.get(key) as T | undefined) ?? initialValue
  );
  const setValue: Dispatch<SetStateAction<T>> = nextValue => {
    setLocalValue(current => {
      const resolved =
        typeof nextValue === "function" ? (nextValue as (current: T) => T)(current) : nextValue;
      cachedState.set(key, resolved);
      return resolved;
    });
  };
  return [value, setValue];
}
