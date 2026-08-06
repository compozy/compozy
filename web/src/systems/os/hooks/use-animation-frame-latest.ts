import { useEffect, useLayoutEffect, useRef, useState } from "react";

interface AnimationFrameLatest<T> {
  cancel: () => void;
  schedule: (event: { value: T }) => void;
}

/** Runs at most once per animation frame with the latest scheduled value. */
export function useAnimationFrameLatest<T>(onFrame: (value: T) => void): AnimationFrameLatest<T> {
  const onFrameRef = useRef(onFrame);
  const frameIDRef = useRef<number | null>(null);
  const pendingRef = useRef<{ value: T } | null>(null);

  useLayoutEffect(() => {
    onFrameRef.current = onFrame;
  }, [onFrame]);

  const [api] = useState<AnimationFrameLatest<T>>(() => ({
    cancel: () => {
      if (frameIDRef.current !== null) cancelAnimationFrame(frameIDRef.current);
      frameIDRef.current = null;
      pendingRef.current = null;
    },
    schedule: event => {
      pendingRef.current = event;
      if (frameIDRef.current !== null) return;
      frameIDRef.current = requestAnimationFrame(() => {
        frameIDRef.current = null;
        const pending = pendingRef.current;
        pendingRef.current = null;
        if (pending !== null) onFrameRef.current(pending.value);
      });
    },
  }));

  useEffect(() => () => api.cancel(), [api]);

  return api;
}
