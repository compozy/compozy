import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  useCreateAutomationJob,
  useCreateAutomationTrigger,
  useDeleteAutomationJob,
  useDeleteAutomationTrigger,
  useTriggerAutomationJob,
  useUpdateAutomationJob,
  useUpdateAutomationTrigger,
} from "@/systems/automation/hooks/use-automation-actions";
import {
  useAcceptAutomationSuggestion,
  useDismissAutomationSuggestion,
} from "@/systems/automation/hooks/use-automation-suggestion-actions";

vi.mock("@/systems/automation/adapters/automation-api", () => ({
  createAutomationJob: vi.fn(),
  updateAutomationJob: vi.fn(),
  deleteAutomationJob: vi.fn(),
  triggerAutomationJob: vi.fn(),
  createAutomationTrigger: vi.fn(),
  updateAutomationTrigger: vi.fn(),
  deleteAutomationTrigger: vi.fn(),
}));

vi.mock("@/systems/automation/adapters/automation-suggestions-api", () => ({
  acceptAutomationSuggestion: vi.fn(),
  dismissAutomationSuggestion: vi.fn(),
}));

import {
  createAutomationJob,
  createAutomationTrigger,
  deleteAutomationJob,
  deleteAutomationTrigger,
  triggerAutomationJob,
  updateAutomationJob,
  updateAutomationTrigger,
} from "@/systems/automation/adapters/automation-api";
import {
  acceptAutomationSuggestion,
  dismissAutomationSuggestion,
} from "@/systems/automation/adapters/automation-suggestions-api";

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
}

describe("automation mutation hooks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("invalidates job and run queries after triggering a job", async () => {
    vi.mocked(triggerAutomationJob).mockResolvedValue({
      id: "run_queued",
      attempt: 1,
      status: "running",
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useTriggerAutomationJob(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "job_1" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(triggerAutomationJob).toHaveBeenCalledWith("job_1");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, { queryKey: ["automation", "jobs"] });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, { queryKey: ["automation", "runs"] });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["automation", "jobs", "detail", "job_1"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(4, {
      queryKey: ["automation", "jobs", "runs", "job_1", "", "", "", ""],
    });
  });

  it("invalidates job list and run list queries after creating a job", async () => {
    vi.mocked(createAutomationJob).mockResolvedValue({
      id: "job_created",
      name: "nightly-docs",
    } as never);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateAutomationJob(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({
        name: "nightly-docs",
        agent_name: "writer",
        prompt: "Summarize docs",
        scope: "workspace",
        workspace_id: "ws_1",
        enabled: true,
        schedule: { mode: "cron", expr: "0 9 * * *" },
        retry: { strategy: "none", max_retries: 3, base_delay: "2s" },
        fire_limit: { max: 12, window: "1h" },
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(createAutomationJob).toHaveBeenCalledWith(
      expect.objectContaining({ name: "nightly-docs" })
    );
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, { queryKey: ["automation", "jobs"] });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, { queryKey: ["automation", "runs"] });
  });

  it("invalidates job detail and run queries after updating a job", async () => {
    vi.mocked(updateAutomationJob).mockResolvedValue({ id: "job_1", enabled: false } as never);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUpdateAutomationJob(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "job_1", data: { enabled: false } });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(updateAutomationJob).toHaveBeenCalledWith("job_1", { enabled: false });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["automation", "jobs", "detail", "job_1"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(4, {
      queryKey: ["automation", "jobs", "runs", "job_1", "", "", "", ""],
    });
  });

  it("invalidates job detail and run queries after deleting a job", async () => {
    vi.mocked(deleteAutomationJob).mockResolvedValue(undefined);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteAutomationJob(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "job_1" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(deleteAutomationJob).toHaveBeenCalledWith("job_1");
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["automation", "jobs", "detail", "job_1"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(4, {
      queryKey: ["automation", "jobs", "runs", "job_1", "", "", "", ""],
    });
  });

  it("invalidates trigger and run queries after deleting a trigger", async () => {
    vi.mocked(deleteAutomationTrigger).mockResolvedValue(undefined);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useDeleteAutomationTrigger(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "trg_1" });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(deleteAutomationTrigger).toHaveBeenCalledWith("trg_1");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["automation", "triggers"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, { queryKey: ["automation", "runs"] });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["automation", "triggers", "detail", "trg_1"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(4, {
      queryKey: ["automation", "triggers", "runs", "trg_1", "", "", "", ""],
    });
  });

  it("invalidates trigger list and run queries after creating a trigger", async () => {
    vi.mocked(createAutomationTrigger).mockResolvedValue({ id: "trg_1" } as never);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useCreateAutomationTrigger(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({
        name: "push-review",
        agent_name: "reviewer",
        prompt: "Review push",
        event: "webhook",
        filter: {},
        scope: "workspace",
        workspace_id: "ws_1",
        enabled: true,
        retry: { strategy: "none", max_retries: 3, base_delay: "2s" },
        fire_limit: { max: 12, window: "1h" },
      });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(createAutomationTrigger).toHaveBeenCalledWith(
      expect.objectContaining({ name: "push-review" })
    );
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      queryKey: ["automation", "triggers"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, { queryKey: ["automation", "runs"] });
  });

  it("invalidates trigger detail and run queries after updating a trigger", async () => {
    vi.mocked(updateAutomationTrigger).mockResolvedValue({ id: "trg_1", enabled: false } as never);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUpdateAutomationTrigger(), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ id: "trg_1", data: { enabled: false } });
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(updateAutomationTrigger).toHaveBeenCalledWith("trg_1", { enabled: false });
    expect(invalidateSpy).toHaveBeenNthCalledWith(3, {
      queryKey: ["automation", "triggers", "detail", "trg_1"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(4, {
      queryKey: ["automation", "triggers", "runs", "trg_1", "", "", "", ""],
    });
  });

  it("invalidates the exact pending suggestions list and job lists after acceptance", async () => {
    vi.mocked(acceptAutomationSuggestion).mockResolvedValue({} as never);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useAcceptAutomationSuggestion("ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ suggestionID: "suggestion_1" });
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(acceptAutomationSuggestion).toHaveBeenCalledWith("ws_alpha", "suggestion_1");
    expect(invalidateSpy).toHaveBeenNthCalledWith(1, {
      exact: true,
      queryKey: ["automation", "suggestions", "list", "ws_alpha", "pending"],
    });
    expect(invalidateSpy).toHaveBeenNthCalledWith(2, {
      queryKey: ["automation", "jobs", "list"],
    });
  });

  it("keeps suggestion and job caches untouched when acceptance fails", async () => {
    vi.mocked(acceptAutomationSuggestion).mockRejectedValue(new Error("accept failed"));
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useAcceptAutomationSuggestion("ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ suggestionID: "suggestion_1" });
    });
    await waitFor(() => expect(result.current.isError).toBe(true));

    expect(invalidateSpy).not.toHaveBeenCalled();
  });

  it("invalidates only the exact pending suggestions list after dismissal", async () => {
    vi.mocked(dismissAutomationSuggestion).mockResolvedValue({} as never);
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDismissAutomationSuggestion("ws_alpha"), {
      wrapper: createWrapper(queryClient),
    });

    act(() => {
      result.current.mutate({ suggestionID: "suggestion_1" });
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidateSpy).toHaveBeenCalledOnce();
    expect(invalidateSpy).toHaveBeenCalledWith({
      exact: true,
      queryKey: ["automation", "suggestions", "list", "ws_alpha", "pending"],
    });
  });
});
