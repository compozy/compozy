import { useEffect, useEffectEvent, useState } from "react";
import type { DependencyList } from "react";

import { runViewSync } from "../reconciler.js";
import { areDependenciesEqual } from "./dependency-list.js";
import { useViewRuntime } from "./use-view-runtime.js";

export interface PromiseMutationOptions<T> {
  optimisticUpdate?: (current: T | undefined) => T;
  rollbackOnError?: boolean;
}

export interface PromiseHookOptions {
  keepPreviousData?: boolean;
}

export interface PromiseResult<T> {
  data: T | undefined;
  isLoading: boolean;
  error: unknown;
  mutate: (options?: PromiseMutationOptions<T>) => Promise<T | undefined>;
}

export function usePromise<T>(
  operation: (signal: AbortSignal) => Promise<T> | T,
  dependencies: DependencyList,
  options: PromiseHookOptions = {}
): PromiseResult<T> {
  const keepPreviousData = options.keepPreviousData !== false;
  const runtime = useViewRuntime();
  const [previousDependencies, setPreviousDependencies] = useState<DependencyList>(dependencies);
  const [generation, setGeneration] = useState(0);
  const runOperation = useEffectEvent(async (signal: AbortSignal) => await operation(signal));
  const [state, setState] = useState<{
    data: T | undefined;
    isLoading: boolean;
    error: unknown;
  }>(() => ({ data: undefined, isLoading: true, error: undefined }));

  if (!areDependenciesEqual(previousDependencies, dependencies)) {
    setPreviousDependencies(dependencies);
    setGeneration(current => current + 1);
    setState(current => ({
      data: keepPreviousData ? current.data : undefined,
      isLoading: true,
      error: undefined,
    }));
  }

  const run = async (options?: PromiseMutationOptions<T>): Promise<T | undefined> => {
    const previous = state.data;
    if (options?.optimisticUpdate) {
      runViewSync(() => {
        setState(current => ({
          data: options.optimisticUpdate?.(current.data),
          isLoading: true,
          error: undefined,
        }));
      });
    }
    try {
      const data = await operation(runtime.signal);
      if (!runtime.signal.aborted) {
        runViewSync(() => {
          setState({ data, isLoading: false, error: undefined });
        });
      }
      return data;
    } catch (error) {
      if (!runtime.signal.aborted) {
        runViewSync(() => {
          setState(current => ({
            data: options?.rollbackOnError ? previous : current.data,
            isLoading: false,
            error,
          }));
        });
      }
      return undefined;
    }
  };

  useEffect(() => {
    const controller = new AbortController();
    const abort = () => controller.abort(runtime.signal.reason);
    runtime.signal.addEventListener("abort", abort, { once: true });
    void Promise.resolve().then(async () => {
      try {
        const data = await runOperation(controller.signal);
        if (!controller.signal.aborted) {
          runViewSync(() => {
            setState({ data, isLoading: false, error: undefined });
          });
        }
      } catch (error) {
        if (!controller.signal.aborted) {
          runViewSync(() => {
            setState(current => ({ ...current, isLoading: false, error }));
          });
        }
      }
    });
    return () => {
      runtime.signal.removeEventListener("abort", abort);
      controller.abort();
    };
  }, [generation, runtime.signal]);

  return { ...state, mutate: run };
}
