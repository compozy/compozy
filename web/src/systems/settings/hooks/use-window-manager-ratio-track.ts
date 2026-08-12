import { useRef, useState, type Dispatch, type PointerEvent, type SetStateAction } from "react";

import { nearestDetent } from "../lib/window-manager-layout-detents";
import { WINDOW_MANAGER_RANGES } from "../lib/window-manager-snap-geometry";
import type { WindowManagerConfig } from "@/systems/os";

const EDGE_PAD = 10;

function clamp(value: number): number {
  const { min, max } = WINDOW_MANAGER_RANGES.repeatRatio;
  return Math.min(Math.max(value, min), max);
}

function round(value: number): number {
  return Math.round(value * 1_000_000) / 1_000_000;
}

function sameRatios(left: readonly number[], right: readonly number[]): boolean {
  return left.length === right.length && left.every((ratio, index) => ratio === right[index]);
}

interface StopIdentityState {
  ratios: readonly number[];
  ids: readonly string[];
  nextSerial: number;
}

function reconcileStopIdentity(
  previous: StopIdentityState,
  nextRatios: readonly number[]
): StopIdentityState {
  const available = previous.ratios.map((ratio, index) => ({
    id: previous.ids[index],
    ratio,
  }));
  let nextSerial = previous.nextSerial;
  const ids = nextRatios.map(ratio => {
    const matchIndex = available.findIndex(candidate => candidate.ratio === ratio);
    if (matchIndex < 0) {
      const id = `repeat-stop-${nextSerial}`;
      nextSerial += 1;
      return id;
    }
    const [match] = available.splice(matchIndex, 1);
    if (!match?.id) throw new Error("Repeat-width stop identity is missing.");
    return match.id;
  });
  return { ratios: [...nextRatios], ids, nextSerial };
}

/**
 * Local interaction state for the repeat-width track. The daemon persists only
 * ratios, so stable UI ids stay here while a stop is dragged through a sibling
 * value and React must retain the focused stop's identity.
 */
export function useWindowManagerRatioTrack(
  draft: WindowManagerConfig,
  setDraft: Dispatch<SetStateAction<WindowManagerConfig>>
) {
  const trackRef = useRef<HTMLDivElement>(null);
  const [selected, setSelected] = useState<number | null>(null);
  const ratios = draft.snap.repeatRatios;
  const limits = WINDOW_MANAGER_RANGES.repeatStops;
  const [identity, setIdentity] = useState<StopIdentityState>(() =>
    reconcileStopIdentity({ ratios: [], ids: [], nextSerial: 0 }, ratios)
  );
  const currentIdentity = sameRatios(identity.ratios, ratios)
    ? identity
    : reconcileStopIdentity(identity, ratios);
  if (currentIdentity !== identity) {
    setIdentity(currentIdentity);
  }
  const stopIdAt = (index: number): string => {
    const id = currentIdentity.ids[index];
    if (!id) throw new Error("Repeat-width stop identity is missing.");
    return id;
  };

  const setRatios = (next: number[], nextIds?: readonly string[]) => {
    setIdentity(
      nextIds
        ? { ...currentIdentity, ratios: [...next], ids: nextIds }
        : reconcileStopIdentity(currentIdentity, next)
    );
    setDraft(current => ({ ...current, snap: { ...current.snap, repeatRatios: next } }));
  };

  const positionFromClientX = (clientX: number): number | null => {
    const rect = trackRef.current?.getBoundingClientRect();
    if (!rect || rect.width <= EDGE_PAD * 2) return null;
    return clamp((clientX - rect.left - EDGE_PAD) / (rect.width - EDGE_PAD * 2));
  };

  const addStop = (event: PointerEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget || ratios.length >= limits.max) return;
    const position = positionFromClientX(event.clientX);
    if (position === null) return;
    const snapped = nearestDetent(position, 0.015) ?? position;
    if (ratios.some(ratio => Math.abs(ratio - snapped) < 0.01)) return;
    const next = [...ratios, round(snapped)];
    setIdentity({
      ratios: next,
      ids: [...currentIdentity.ids, `repeat-stop-${currentIdentity.nextSerial}`],
      nextSerial: currentIdentity.nextSerial + 1,
    });
    setDraft(current => ({ ...current, snap: { ...current.snap, repeatRatios: next } }));
    setSelected(ratios.length);
  };

  return {
    addStop,
    limits,
    positionFromClientX,
    preview: ratios[selected ?? 0] ?? 0.5,
    ratios,
    removeStop: (index: number) => {
      setRatios(
        ratios.filter((_, at) => at !== index),
        currentIdentity.ids.filter((_, at) => at !== index)
      );
      setSelected(null);
    },
    selected,
    selectStop: setSelected,
    setRatioAt: (index: number, next: number) =>
      setRatios(
        ratios.map((value, at) => (at === index ? next : value)),
        [...currentIdentity.ids]
      ),
    stopIdAt,
    trackRef,
  };
}
