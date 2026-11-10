"use client";

import { useState } from "react";

interface Backed<T> {
  value: T;
  /** The URL value this state last agreed with. */
  seen: T;
  /** Values this component wrote to the URL and has not yet seen come back. */
  echoes: readonly T[];
}

/**
 * A value that lives in the URL but is edited locally first (typing in a search box). The URL is
 * the source of truth: when it changes for a reason other than this component's own writes (a
 * navigation, the header link), the local value follows it. Its own writes come back as echoes and
 * are ignored, so a slow URL update never overwrites newer typing.
 */
export function useUrlBackedValue<T>(urlValue: T): [T, (next: T) => void] {
  const [state, setState] = useState<Backed<T>>({ value: urlValue, seen: urlValue, echoes: [] });
  if (!Object.is(state.seen, urlValue)) {
    const echo = state.echoes.indexOf(urlValue);
    setState(
      echo === -1
        ? { value: urlValue, seen: urlValue, echoes: [] }
        : { ...state, seen: urlValue, echoes: state.echoes.slice(echo + 1) }
    );
  }
  const set = (next: T) =>
    setState(current => ({ ...current, value: next, echoes: [...current.echoes, next] }));
  return [state.value, set];
}
