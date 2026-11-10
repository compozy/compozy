"use client";

import { useState } from "react";

interface Backed<T> {
  value: T;
  /** The URL value this state last agreed with. */
  seen: T;
  /** Values this component wrote to the URL, oldest first, that the URL may still report. */
  echoes: readonly T[];
}

/** Folds the URL's current value into the state; returns `state` itself when nothing changes. */
function reconcile<T>(state: Backed<T>, url: T): Backed<T> {
  let { value, seen, echoes } = state;
  if (!Object.is(seen, url)) {
    const echo = echoes.findIndex(written => Object.is(written, url));
    // Not one of our writes: a navigation (the header link, Back). The URL wins.
    if (echo === -1) return { value: url, seen: url, echoes: [] };
    // A late report of our own write: it and every older write are done.
    seen = url;
    echoes = echoes.slice(echo + 1);
  }
  // The URL already reads our latest write, so nothing older can still come back.
  if (echoes.length > 0 && Object.is(echoes.at(-1), url)) echoes = [];
  if (Object.is(seen, state.seen) && echoes === state.echoes) return state;
  return { value, seen, echoes };
}

/**
 * A value that lives in the URL but is edited locally first (typing in a search box). The URL is
 * the source of truth: when it changes for a reason other than this component's own writes (a
 * navigation, the header link), the local value follows it. Own writes reach the URL in order but
 * may be reported late or batched together (the router applies every pending write in one render),
 * so a report of an older write never overwrites newer typing, and writes the URL has caught up
 * with are forgotten: a later navigation to one of those values still applies.
 */
export function useUrlBackedValue<T>(urlValue: T): [T, (next: T) => void] {
  const [state, setState] = useState<Backed<T>>({ value: urlValue, seen: urlValue, echoes: [] });
  const current = reconcile(state, urlValue);
  if (current !== state) setState(current);
  const set = (next: T) =>
    setState(previous => ({ ...previous, value: next, echoes: [...previous.echoes, next] }));
  return [current.value, set];
}
