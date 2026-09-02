import { useEffect, useRef } from "react";

export function useAbortableMutationRequest() {
  const controllersRef = useRef<Set<AbortController> | null>(null);
  if (controllersRef.current === null) controllersRef.current = new Set();

  useEffect(
    () => () => {
      const controllers = controllersRef.current;
      if (controllers === null) return;
      for (const controller of controllers) controller.abort();
      controllers.clear();
    },
    []
  );

  return async <T>(request: (signal: AbortSignal) => Promise<T>): Promise<T> => {
    const controllers = controllersRef.current;
    if (controllers === null) {
      throw new Error("Session mutation controller registry is unavailable");
    }
    const controller = new AbortController();
    controllers.add(controller);
    return Promise.resolve()
      .then(() => request(controller.signal))
      .finally(() => controllers.delete(controller));
  };
}
