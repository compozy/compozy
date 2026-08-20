import { useLayoutEffect, useRef, useState } from "react";
import type { DependencyList } from "react";

import { runViewSync } from "../reconciler.js";
import { areDependenciesEqual, identityCacheKey } from "./dependency-list.js";
import { usePromise } from "./use-promise.js";
import type { PromiseHookOptions, PromiseResult } from "./use-promise.js";
import { useViewRuntime } from "./use-view-runtime.js";

const pageCache = new WeakMap<Function, Map<string, unknown>>();

export interface CachedPromiseOptions extends PromiseHookOptions {}

export interface CachedPromiseResult<T> extends PromiseResult<T> {
  loadMore: () => Promise<T | undefined>;
}

export function useCachedPromise<T>(
  operation: (signal: AbortSignal, page: number) => Promise<T> | T,
  dependencies: DependencyList,
  options: CachedPromiseOptions = {}
): CachedPromiseResult<T> {
  const keepPreviousData = options.keepPreviousData !== false;
  const runtime = useViewRuntime();
  const latestDependencies = useRef(dependencies);
  const [pagination, setPagination] = useState<{
    dependencies: DependencyList;
    page: number;
    extra: T | undefined;
    loadingMore: boolean;
    loadError: unknown;
  }>(() => ({
    dependencies,
    page: 0,
    extra: undefined,
    loadingMore: false,
    loadError: undefined,
  }));

  if (!areDependenciesEqual(pagination.dependencies, dependencies)) {
    setPagination({
      dependencies,
      page: 0,
      extra: undefined,
      loadingMore: false,
      loadError: undefined,
    });
  }

  useLayoutEffect(() => {
    latestDependencies.current = dependencies;
  });

  const result = usePromise(
    async signal => {
      const cacheKey = identityCacheKey(dependencies);
      const cached = readPageCache(operation, cacheKey, 0);
      if (cached !== undefined) return cached as T;
      const data = await operation(signal, 0);
      writePageCache(operation, cacheKey, 0, data);
      return data;
    },
    dependencies,
    { keepPreviousData }
  );

  const loadMore = async (): Promise<T | undefined> => {
    const requestDependencies = dependencies;
    const nextPage = pagination.page + 1;
    const baseData = pagination.extra ?? result.data;
    const cacheKey = identityCacheKey(requestDependencies);
    runViewSync(() => {
      setPagination(current =>
        areDependenciesEqual(current.dependencies, requestDependencies)
          ? { ...current, loadingMore: true, loadError: undefined }
          : current
      );
    });
    try {
      const cached = readPageCache(operation, cacheKey, nextPage);
      const next = cached !== undefined ? (cached as T) : await operation(runtime.signal, nextPage);
      if (
        runtime.signal.aborted ||
        !areDependenciesEqual(latestDependencies.current, requestDependencies)
      ) {
        if (!runtime.signal.aborted) {
          runViewSync(() => {
            setPagination(current =>
              areDependenciesEqual(current.dependencies, requestDependencies)
                ? { ...current, loadingMore: false }
                : current
            );
          });
        }
        return undefined;
      }
      writePageCache(operation, cacheKey, nextPage, next);
      runViewSync(() => {
        setPagination(current =>
          areDependenciesEqual(current.dependencies, requestDependencies)
            ? {
                ...current,
                page: nextPage,
                extra: mergePage(current.extra ?? baseData, next),
                loadingMore: false,
              }
            : current
        );
      });
      return next;
    } catch (error) {
      if (!runtime.signal.aborted) {
        runViewSync(() => {
          setPagination(current =>
            areDependenciesEqual(current.dependencies, requestDependencies)
              ? { ...current, loadError: error, loadingMore: false }
              : current
          );
        });
      }
      return undefined;
    }
  };

  return {
    data: pagination.extra ?? result.data,
    isLoading: result.isLoading || pagination.loadingMore,
    error: pagination.loadError ?? result.error,
    mutate: result.mutate,
    loadMore,
  };
}

function readPageCache(operation: Function, cacheKey: string, page: number): unknown {
  return pageCache.get(operation)?.get(pageCacheKey(cacheKey, page));
}

function writePageCache(operation: Function, cacheKey: string, page: number, data: unknown): void {
  let pages = pageCache.get(operation);
  if (!pages) {
    pages = new Map();
    pageCache.set(operation, pages);
  }
  pages.set(pageCacheKey(cacheKey, page), data);
}

function pageCacheKey(cacheKey: string, page: number): string {
  return `${cacheKey}:${page}`;
}

function mergePage<T>(current: T | undefined, next: T): T {
  if (Array.isArray(current) && Array.isArray(next)) {
    return [...current, ...next] as T;
  }
  return next;
}
