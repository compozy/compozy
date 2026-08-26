import { useRef, useState } from "react";
import type { Dispatch, SetStateAction } from "react";

const cachedState = new Map<string, unknown>();

export function useCachedState<T>(key: string, initialValue: T): [T, Dispatch<SetStateAction<T>>] {
  const [value, setLocalValue] = useState<T>(
    () => (cachedState.get(key) as T | undefined) ?? initialValue
  );
  const valueRef = useRef(value);
  const setValue: Dispatch<SetStateAction<T>> = nextValue => {
    const resolved =
      typeof nextValue === "function"
        ? (nextValue as (current: T) => T)(valueRef.current)
        : nextValue;
    valueRef.current = resolved;
    cachedState.set(key, resolved);
    setLocalValue(resolved);
  };
  return [value, setValue];
}
