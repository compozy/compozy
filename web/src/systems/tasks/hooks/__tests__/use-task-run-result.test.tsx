// Suite: paged task-run result controller
// Invariant: external bytes stay unfetched while closed, and copy joins every exact page before
// decoding UTF-8 once.
// Owning layer: task result query controller. Boundary OUT: paged task result adapter.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { readTaskRunResult } from "@/systems/tasks/adapters/tasks-api";

import { useTaskRunResult } from "../use-task-run-result";

vi.mock("@/systems/tasks/adapters/tasks-api", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/tasks/adapters/tasks-api")>();
  return { ...actual, readTaskRunResult: vi.fn() };
});

const RUN_ID = "run_large";
const RESULT_REF = "sha256:large";
const WORKSPACE_ID = "ws_alpha";
const PAGE_BYTES = 16 * 1024;

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  };
}

function encodeBase64(bytes: Uint8Array): string {
  let binary = "";
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return globalThis.btoa(binary);
}

describe("useTaskRunResult", () => {
  const source = `${"a".repeat(PAGE_BYTES - 1)}界`;
  const sourceBytes = new TextEncoder().encode(source);
  const writeText = vi.fn<(_value: string) => Promise<void>>();

  beforeEach(() => {
    vi.clearAllMocks();
    writeText.mockResolvedValue();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    vi.mocked(readTaskRunResult).mockImplementation((_id, offset, limit) => {
      const end = Math.min(sourceBytes.byteLength, offset + limit);
      const pageBytes = sourceBytes.slice(offset, end);
      return Promise.resolve({
        run_id: RUN_ID,
        result_ref: RESULT_REF,
        offset,
        bytes: pageBytes.byteLength,
        total_bytes: sourceBytes.byteLength,
        data_base64: encodeBase64(pageBytes),
        ...(end < sourceBytes.byteLength ? { next_offset: end } : {}),
        eof: end === sourceBytes.byteLength,
      });
    });
  });

  it("Should defer reads until open and copy multibyte content across page boundaries", async () => {
    const { result } = renderHook(
      () =>
        useTaskRunResult({
          resultBytes: sourceBytes.byteLength,
          resultRef: RESULT_REF,
          runId: RUN_ID,
          workspaceId: WORKSPACE_ID,
        }),
      { wrapper: createWrapper() }
    );

    expect(readTaskRunResult).not.toHaveBeenCalled();

    act(() => result.current.onOpenChange(true));
    await waitFor(() => expect(result.current.page?.offset).toBe(0));
    expect(readTaskRunResult).toHaveBeenCalledWith(RUN_ID, 0, PAGE_BYTES, expect.any(AbortSignal));

    await act(async () => result.current.onCopy());

    expect(writeText).toHaveBeenCalledWith(source);
    expect(result.current.copyState).toBe("copied");
    expect(readTaskRunResult).toHaveBeenCalledWith(
      RUN_ID,
      PAGE_BYTES,
      PAGE_BYTES,
      expect.any(AbortSignal)
    );
  });
});
