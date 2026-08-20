import type { DependencyList } from "react";

import { usePromise } from "./use-promise.js";
import type { PromiseResult } from "./use-promise.js";

const promiseCache = new WeakMap<Function, Map<string, unknown>>();

export interface CachedPromiseOptions {
  keepPreviousData?: boolean;
}

export interface CachedPromiseResult<T> extends PromiseResult<T> {
  loadMore: () => Promise<T | undefined>;
}

export function useCachedPromise<T>(
  operation: (signal: AbortSignal) => Promise<T> | T,
  dependencies: DependencyList,
  _options: CachedPromiseOptions = {}
): CachedPromiseResult<T> {
  const key = stableCacheKey(dependencies);
  const result = usePromise(async signal => {
    let operationCache = promiseCache.get(operation);
    if (!operationCache) {
      operationCache = new Map();
      promiseCache.set(operation, operationCache);
    }
    if (operationCache.has(key)) {
      return operationCache.get(key) as T;
    }
    const data = await operation(signal);
    operationCache.set(key, data);
    return data;
  }, dependencies);
  return { ...result, loadMore: async () => await result.mutate() };
}

function stableCacheKey(dependencies: DependencyList): string {
  return JSON.stringify(dependencies, (_key, value: unknown) => {
    if (typeof value === "function") {
      return "[function]";
    }
    return value;
  });
}
