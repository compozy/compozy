# System Implementation Patterns

> **Note on project-specific imports**: The patterns below use placeholder imports (`your-http-client`, `your-query-client`, etc.). Replace these with the actual paths used in the target project. Before writing code, read the existing systems in the codebase to confirm the exact import paths for the HTTP client, query client, error base class, and other shared utilities.

---

## API Service Layer (`adapters/<domain>-api.ts`)

The adapter owns all HTTP communication for the domain. It exports a single namespace object and a typed error class. All internal helpers stay private.

```ts
import type { FooResponse, CreateFooBody, UpdateFooBody } from "../types";

// --- Typed error class ---
// Extend the project's base error class if one exists (e.g. AppError, ApiError).
// Otherwise extend Error directly.
export class FooApiError extends Error {
  constructor(
    message: string,
    public readonly statusCode?: number,
    public readonly code?: string
  ) {
    super(message);
    this.name = "FooApiError";
  }
}

// --- Private helpers ---
function extractErrorMessage(error: unknown, fallback: string): string {
  if (!error || typeof error !== "object") return fallback;
  const e = error as Record<string, unknown>;
  // Handle both flat { message } and nested { error: { message } } shapes
  const nested = (e.error as Record<string, unknown> | undefined)?.message;
  const flat = e.message;
  return (typeof nested === "string" ? nested : typeof flat === "string" ? flat : null) ?? fallback;
}

async function handleResponse<T>(response: Response, fallback: string): Promise<T> {
  if (!response.ok) {
    let body: unknown;
    try {
      body = await response.json();
    } catch {
      /* ignore */
    }
    throw new FooApiError(extractErrorMessage(body, fallback), response.status);
  }
  if (response.status === 204) return null as T;
  return response.json() as Promise<T>;
}

// --- API namespace ---
// Use the project's typed HTTP client (e.g. openapi-fetch, ky, axios, or raw fetch).
// The important contract: accept signal, throw FooApiError on failure.

export const fooApi = {
  list: async (scopeId: string, signal?: AbortSignal): Promise<FooResponse[]> => {
    const response = await fetch(`/api/foos?scopeId=${scopeId}`, { signal });
    return handleResponse<FooResponse[]>(response, "Failed to fetch foos");
  },

  get: async (fooId: string, signal?: AbortSignal): Promise<FooResponse> => {
    const response = await fetch(`/api/foos/${fooId}`, { signal });
    return handleResponse<FooResponse>(response, "Failed to fetch foo");
  },

  create: async (body: CreateFooBody): Promise<FooResponse> => {
    const response = await fetch("/api/foos", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return handleResponse<FooResponse>(response, "Failed to create foo");
  },

  update: async (fooId: string, body: UpdateFooBody): Promise<FooResponse> => {
    const response = await fetch(`/api/foos/${fooId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    return handleResponse<FooResponse>(response, "Failed to update foo");
  },

  delete: async (fooId: string, signal?: AbortSignal): Promise<void> => {
    const response = await fetch(`/api/foos/${fooId}`, { method: "DELETE", signal });
    await handleResponse<null>(response, "Failed to delete foo");
  },
};
```

---

## Query Keys (`lib/query-keys.ts`)

Use hierarchical key structure for granular invalidation. Each level enables broader or narrower cache operations.

```ts
export const fooKeys = {
  all: ["foo"] as const,
  // Granular levels for targeted invalidation
  lists: () => [...fooKeys.all, "list"] as const,
  list: (scopeId: string | null) => [...fooKeys.lists(), scopeId] as const,
  details: () => [...fooKeys.all, "detail"] as const,
  detail: (fooId: string) => [...fooKeys.details(), fooId] as const,
};

// Invalidation examples:
// queryClient.invalidateQueries({ queryKey: fooKeys.all })       — invalidates everything
// queryClient.invalidateQueries({ queryKey: fooKeys.lists() })   — invalidates all lists
// queryClient.invalidateQueries({ queryKey: fooKeys.list("x") }) — invalidates one specific list
```

---

## Query Options (`lib/query-options.ts`)

Co-locate `queryKey` and `queryFn` via `queryOptions` for type safety, reuse across hooks, prefetching, and route loaders.

```ts
import { queryOptions } from "@tanstack/react-query";
import { fooApi } from "../adapters/foo-api";
import { fooKeys } from "./query-keys";

const STALE_TIME = 1000 * 60; // 1 min

export function fooListOptions(scopeId: string | null) {
  return queryOptions({
    queryKey: fooKeys.list(scopeId),
    queryFn: ({ signal }) => fooApi.list(scopeId!, signal),
    staleTime: STALE_TIME,
    enabled: Boolean(scopeId),
  });
}

export function fooDetailOptions(fooId: string) {
  return queryOptions({
    queryKey: fooKeys.detail(fooId),
    queryFn: ({ signal }) => fooApi.get(fooId, signal),
    enabled: Boolean(fooId),
  });
}
```

Usage across the system:

```ts
// In a hook
useQuery(fooListOptions(scopeId));

// In a route loader for prefetching
queryClient.ensureQueryData(fooListOptions(scopeId));

// For manual cache reads
queryClient.getQueryData(fooListOptions(scopeId).queryKey);
```

---

## Zod Schema (`lib/<domain>-schemas.ts`)

```ts
import { z } from "zod";

const fooStatusSchema = z.enum(["pending", "active", "completed"]);

export const fooSchema = z.object({
  id: z.string(),
  title: z.string(),
  status: fooStatusSchema,
  createdAt: z.string().datetime(),
  updatedAt: z.string().datetime(),
});

export type FooStatus = z.infer<typeof fooStatusSchema>;
```

---

## Query Hook (`hooks/use-<action>.ts`)

Wrap `queryOptions` in a hook. Accept scope parameters and optional overrides.

```ts
import { useQuery } from "@tanstack/react-query";
import { fooListOptions, fooDetailOptions } from "../lib/query-options";

export function useFooList(scopeId: string | null, options?: { enabled?: boolean }) {
  const enabled = Boolean(scopeId) && (options?.enabled ?? true);
  return useQuery({ ...fooListOptions(scopeId), enabled });
}

export function useFooDetail(fooId: string) {
  return useQuery(fooDetailOptions(fooId));
}
```

---

## Mutation Hook — Simple (`hooks/use-create-<entity>.ts`)

Use `useMutation` with `onSettled` invalidation. This is the standard pattern when optimistic updates are not needed.

```ts
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { fooApi } from "../adapters/foo-api";
import { fooKeys } from "../lib/query-keys";
import type { CreateFooBody, FooResponse } from "../types";

export function useCreateFoo(scopeId: string) {
  const queryClient = useQueryClient();

  return useMutation<FooResponse, Error, CreateFooBody>({
    mutationFn: body => fooApi.create(body),
    onSuccess: () => {
      // Invalidate the list to refetch fresh data
      queryClient.invalidateQueries({ queryKey: fooKeys.list(scopeId) });
    },
  });
}
```

---

## Mutation Hook — Optimistic via UI Variables (`hooks/use-update-<entity>.ts`)

The simplest optimistic pattern. Use when the optimistic change is only visible in one place. No cache manipulation or rollback needed.

```ts
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { fooApi } from "../adapters/foo-api";
import { fooKeys } from "../lib/query-keys";
import type { UpdateFooBody, FooResponse } from "../types";

export function useUpdateFoo(scopeId: string) {
  const queryClient = useQueryClient();

  return useMutation<FooResponse, Error, { fooId: string; body: UpdateFooBody }>({
    mutationFn: ({ fooId, body }) => fooApi.update(fooId, body),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: fooKeys.list(scopeId) });
    },
  });
}

// In the component — use `variables` and `isPending` for optimistic UI:
//
// const { mutate, variables, isPending } = useUpdateFoo(scopeId);
// const displayTitle = isPending ? variables?.body.title : foo.title;
```

---

## Mutation Hook — Optimistic via Cache (`hooks/use-create-<entity>-optimistic.ts`)

Use cache-based optimistic updates when multiple components need to reflect the change immediately.

```ts
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { fooApi } from "../adapters/foo-api";
import { fooKeys } from "../lib/query-keys";
import type { CreateFooBody, FooResponse } from "../types";

export function useCreateFooOptimistic(scopeId: string) {
  const queryClient = useQueryClient();
  const listKey = fooKeys.list(scopeId);

  return useMutation<
    FooResponse,
    Error,
    CreateFooBody,
    { previousFoos: FooResponse[] | undefined }
  >({
    mutationFn: body => fooApi.create(body),

    onMutate: async newFoo => {
      // 1. Cancel outgoing refetches to prevent them from overwriting optimistic data
      await queryClient.cancelQueries({ queryKey: listKey });

      // 2. Snapshot previous data for rollback
      const previousFoos = queryClient.getQueryData<FooResponse[]>(listKey);

      // 3. Optimistically add the new item to the cache
      queryClient.setQueryData<FooResponse[]>(listKey, old => [
        ...(old ?? []),
        {
          id: crypto.randomUUID(), // Temporary ID
          ...newFoo,
          status: newFoo.status ?? "pending",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        } as FooResponse,
      ]);

      // 4. Return snapshot for rollback
      return { previousFoos };
    },

    onError: (_err, _newFoo, context) => {
      // Roll back to the previous state on failure
      if (context?.previousFoos) {
        queryClient.setQueryData<FooResponse[]>(listKey, context.previousFoos);
      }
    },

    onSettled: () => {
      // Always invalidate to sync with the server
      queryClient.invalidateQueries({ queryKey: listKey });
    },
  });
}
```

---

## Delete Mutation Hook (`hooks/use-delete-<entity>.ts`)

```ts
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { fooApi } from "../adapters/foo-api";
import { fooKeys } from "../lib/query-keys";
import type { FooResponse } from "../types";

export function useDeleteFoo(scopeId: string) {
  const queryClient = useQueryClient();
  const listKey = fooKeys.list(scopeId);

  return useMutation<void, Error, string, { previousFoos: FooResponse[] | undefined }>({
    mutationFn: fooId => fooApi.delete(fooId),

    onMutate: async fooId => {
      await queryClient.cancelQueries({ queryKey: listKey });
      const previousFoos = queryClient.getQueryData<FooResponse[]>(listKey);

      // Optimistically remove from the list
      queryClient.setQueryData<FooResponse[]>(
        listKey,
        old => old?.filter(item => item.id !== fooId) ?? []
      );

      return { previousFoos };
    },

    onError: (_err, _fooId, context) => {
      if (context?.previousFoos) {
        queryClient.setQueryData<FooResponse[]>(listKey, context.previousFoos);
      }
    },

    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: listKey });
    },
  });
}
```

---

## Context (`contexts/<domain>-context.tsx`)

Use context to share query data or domain state across a component subtree without prop-drilling.

```ts
import { createContext, use, useMemo } from "react";
import type { FooResponse } from "../types";

interface FooContextValue {
  scopeId: string;
  items: FooResponse[];
  isLoading: boolean;
}

export const FooContext = createContext<FooContextValue | null>(null);

export function FooProvider({
  scopeId,
  items,
  isLoading,
  children,
}: FooContextValue & { children: React.ReactNode }) {
  const value = useMemo(() => ({ scopeId, items, isLoading }), [scopeId, items, isLoading]);
  return <FooContext value={value}>{children}</FooContext>;
}

// Consumer hook — throws descriptively if used outside the provider
export function useFooContext(): FooContextValue {
  const ctx = use(FooContext);
  if (!ctx) throw new Error("useFooContext must be used within <FooProvider>");
  return ctx;
}
```

For performance-sensitive trees with many consumers, split into focused sub-contexts:

```ts
export const FooCoreContext    = createContext<FooCoreValue | null>(null);
export const FooUiContext      = createContext<FooUiValue | null>(null);
export const FooActionsContext = createContext<FooActionsValue | null>(null);

export function FooProvider({ children, ...props }: FooProviderProps) {
  const state = useFooProviderState(props);
  return (
    <FooCoreContext value={state.coreValue}>
      <FooUiContext value={state.uiValue}>
        <FooActionsContext value={state.actionsValue}>
          {children}
        </FooActionsContext>
      </FooUiContext>
    </FooCoreContext>
  );
}
```

---

## XState Store (`stores/<domain>-store.ts`)

Use for complex async state machines — multi-step flows, polling, event emission.

```ts
import { createStore } from "@xstate/store";
import { fooApi } from "../adapters/foo-api";
import type { FooResponse } from "../types";

interface FooStoreContext {
  data: FooResponse | null;
  isLoading: boolean;
  error: string | null;
}

export const fooStore = createStore({
  context: {
    data: null,
    isLoading: false,
    error: null,
  } as FooStoreContext,

  emits: {
    dataLoaded: (_payload: { data: FooResponse }) => {},
    loadFailed: (_payload: { error: string }) => {},
  },

  on: {
    load: (context, event: { id: string }, enqueue) => {
      enqueue.effect(async () => {
        try {
          const data = await fooApi.get(event.id);
          enqueue.emit.dataLoaded({ data });
          fooStore.trigger.setData({ data });
        } catch (err) {
          const error = err instanceof Error ? err.message : "Failed to load";
          enqueue.emit.loadFailed({ error });
          fooStore.trigger.setError({ error });
        }
      });
      return { ...context, isLoading: true, error: null };
    },

    setData: (context, event: { data: FooResponse }) => ({
      ...context,
      data: event.data,
      isLoading: false,
      error: null,
    }),

    setError: (context, event: { error: string }) => ({
      ...context,
      error: event.error,
      isLoading: false,
    }),

    reset: _context => ({ data: null, isLoading: false, error: null }),
  },
});
```

---

## View-Model Hook

Compose multiple hooks for a single page/shell component. Return a flat object.

```ts
import { useFooList } from "./use-foo-list";
import { useFooDetail } from "./use-foo-detail";
import { useCreateFoo } from "./use-create-foo";
import { useDeleteFoo } from "./use-delete-foo";

export function useFooDetailViewModel(fooId: string, scopeId: string) {
  const list = useFooList(scopeId);
  const { data: detail, isLoading, error } = useFooDetail(fooId);
  const create = useCreateFoo(scopeId);
  const remove = useDeleteFoo(scopeId);

  return { detail, isLoading, error, items: list.data, create, remove };
}
```

---

## Types (`types.ts`)

Export clean domain types. Derive from the project's API contract when available.

```ts
// If the project has a generated API contract, derive from it:
// import type { OperationResponse } from "your-api-contract";
// type FooContract = OperationResponse<"getFoo">;
// export type FooResponse = FooContract;

// Otherwise define types directly:
export interface FooResponse {
  id: string;
  title: string;
  status: FooStatus;
  createdAt: string;
  updatedAt: string;
}

export type FooStatus = "pending" | "active" | "completed";

export interface CreateFooBody {
  title: string;
  status?: FooStatus;
}

export interface UpdateFooBody {
  title?: string;
  status?: FooStatus;
}

/**
 * Aggregated UI state for the foo detail view.
 *
 * Derivation rules:
 * - isActive: status === "active"
 * - canEdit: isActive && !isDeleting
 */
export interface FooDetailState {
  foo: FooResponse;
  isActive: boolean;
  canEdit: boolean;
}
```
