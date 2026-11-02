import { QueryClient, QueryClientProvider, QueryObserver } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { drainScheduler, pauseScheduler, resumeScheduler } from "../../adapters/scheduler-api";
import { schedulerKeys } from "../../lib/query-keys";
import { useDrainScheduler, usePauseScheduler, useResumeScheduler } from "../use-scheduler-actions";

vi.mock("../../adapters/scheduler-api", () => ({
  drainScheduler: vi.fn(),
  pauseScheduler: vi.fn(),
  resumeScheduler: vi.fn(),
}));

describe("scheduler mutation hooks", () => {
  for (const [action, useAction, api] of [
    ["pause", usePauseScheduler, pauseScheduler],
    ["resume", useResumeScheduler, resumeScheduler],
    ["drain", useDrainScheduler, drainScheduler],
  ] as const) {
    it.each(["success", "failure"])(
      `Should reconcile every scheduler read after ${action} %s`,
      async outcome => {
        const queryClient = new QueryClient({
          defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
        });
        const keys = [
          schedulerKeys.status(),
          schedulerKeys.backlog({ workspace: "ws-1" }),
          schedulerKeys.backlog({ workspace: "ws-2" }),
        ];
        const reads = [0, 0, 0];
        let canceled = 0;
        const observers = keys.map((queryKey, index) => {
          queryClient.setQueryData(queryKey, { revision: 0 });
          return new QueryObserver(queryClient, {
            queryKey,
            staleTime: Infinity,
            queryFn: ({ signal }) => {
              reads[index]++;
              signal.addEventListener(
                "abort",
                () => {
                  canceled++;
                },
                { once: true }
              );
              return Promise.resolve({ revision: 1 });
            },
          });
        });
        const unsubscribers = observers.map(observer => observer.subscribe(() => undefined));
        queryClient.setQueryData(["other-system"], { revision: 0 });
        if (outcome === "success") {
          vi.mocked(api).mockResolvedValue({} as never);
        } else {
          vi.mocked(api).mockRejectedValue(new Error("Action failed"));
        }
        const { result, unmount } = renderHook(() => useAction(), {
          wrapper: ({ children }: { children: ReactNode }) =>
            createElement(QueryClientProvider, { client: queryClient }, children),
        });
        try {
          await act(async () => {
            await result.current.mutateAsync({}).catch(() => undefined);
          });
          for (const observer of observers) {
            expect(observer.getCurrentResult().data).toEqual({ revision: 1 });
          }
          expect(queryClient.getQueryState(["other-system"])?.isInvalidated).toBe(false);
          console.log(`[scheduler-action] ${JSON.stringify({ action, outcome, reads, canceled })}`);
          expect(reads).toEqual([1, 1, 1]);
          expect(canceled).toBe(0);
        } finally {
          unmount();
          for (const unsubscribe of unsubscribers) unsubscribe();
          queryClient.clear();
        }
      }
    );
  }
});
