import { useRef, useState, type Dispatch, type PointerEvent, type SetStateAction } from "react";

import type { WindowManagerConfig } from "@/systems/os";

import { nearestDetent } from "../lib/window-manager-layout-detents";
import { WINDOW_MANAGER_RANGES } from "../lib/window-manager-snap-geometry";

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

function reconcileStopIds(
  previousRatios: readonly number[],
  previousIds: readonly string[],
  nextRatios: readonly number[],
  nextId: () => string
): string[] {
  const available = previousRatios.map((ratio, index) => ({ id: previousIds[index], ratio }));
  return nextRatios.map(ratio => {
    const matchIndex = available.findIndex(candidate => candidate.ratio === ratio);
    if (matchIndex < 0) return nextId();
    const [match] = available.splice(matchIndex, 1);
    if (!match?.id) throw new Error("Repeat-width stop identity is missing.");
    return match.id;
  });
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
  const knownRatiosRef = useRef<readonly number[]>([]);
  const stopIdsRef = useRef<readonly string[]>([]);
  const nextStopIdRef = useRef(0);
  const [selected, setSelected] = useState<number | null>(null);
  const ratios = draft.snap.repeatRatios;
  const limits = WINDOW_MANAGER_RANGES.repeatStops;

  const nextStopId = () => {
    const id = `repeat-stop-${nextStopIdRef.current}`;
    nextStopIdRef.current += 1;
    return id;
  };
  if (!sameRatios(knownRatiosRef.current, ratios)) {
    stopIdsRef.current = reconcileStopIds(
      knownRatiosRef.current,
      stopIdsRef.current,
      ratios,
      nextStopId
    );
    knownRatiosRef.current = [...ratios];
  }
  const stopIdAt = (index: number): string => {
    const id = stopIdsRef.current[index];
    if (!id) throw new Error("Repeat-width stop identity is missing.");
    return id;
  };

  const setRatios = (
    next: number[],
    nextIds = reconcileStopIds(knownRatiosRef.current, stopIdsRef.current, next, nextStopId)
  ) => {
    knownRatiosRef.current = [...next];
    stopIdsRef.current = nextIds;
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
    setRatios([...ratios, round(snapped)], [...stopIdsRef.current, nextStopId()]);
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
        stopIdsRef.current.filter((_, at) => at !== index)
      );
      setSelected(null);
    },
    selected,
    selectStop: setSelected,
    setRatioAt: (index: number, next: number) =>
      setRatios(
        ratios.map((value, at) => (at === index ? next : value)),
        [...stopIdsRef.current]
      ),
    stopIdAt,
    trackRef,
  };
}
