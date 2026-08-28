// Suite: terminal recordings host hook
// Invariant: elapsed is derived from `at`; one timer runs while the map is
// non-empty, is cleared when the map empties or the host unmounts, and is
// created exactly once more when the map is written again after a clear.
// Boundary IN: useTerminalRecordings over the scoped recordings cache.
// Boundary OUT: catalog stream writes, owned by use-terminal-catalog-stream.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { terminalKeys } from "../../lib/query-keys";
import {
  applyRecordingStopSuccess,
  type TerminalRecordingMap,
} from "../../lib/terminal-recording-state";
import { useTerminalRecordings } from "../use-terminal-recordings";

const SCOPE = { workspaceId: "ws-atlas", profileKey: "work" };
const AT = "2026-08-25T12:00:00.000Z";
const START_MS = Date.parse(AT);

function renderRecordings(client: QueryClient) {
  return renderHook(() => useTerminalRecordings(SCOPE, true), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    ),
  });
}

describe("useTerminalRecordings", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(START_MS);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should derive elapsed from at and tick once per second while recording", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData<TerminalRecordingMap>(terminalKeys.recordings(SCOPE), {
      "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    });
    const interval = vi.spyOn(window, "setInterval");
    const { result } = renderRecordings(client);

    expect(result.current["term-4f21c9a03b7e"]?.elapsed).toBe("00:00");
    expect(interval).toHaveBeenCalledOnce();

    act(() => {
      vi.advanceTimersByTime(134_000);
    });
    expect(result.current["term-4f21c9a03b7e"]?.elapsed).toBe("02:14");
  });

  it("Should clear its timer when the map empties and on unmount", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData<TerminalRecordingMap>(terminalKeys.recordings(SCOPE), {
      "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    });
    const clear = vi.spyOn(window, "clearInterval");
    const { unmount } = renderRecordings(client);

    act(() => {
      client.setQueryData<TerminalRecordingMap>(terminalKeys.recordings(SCOPE), {});
    });
    expect(clear).toHaveBeenCalled();
    unmount();

    client.setQueryData<TerminalRecordingMap>(terminalKeys.recordings(SCOPE), {
      "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    });
    const again = renderRecordings(client);
    again.unmount();
    expect(clear.mock.calls.length).toBeGreaterThan(1);
  });

  it("Should own exactly one timer after the map is cleared and written again", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = terminalKeys.recordings(SCOPE);
    client.setQueryData<TerminalRecordingMap>(key, {
      "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    });
    const interval = vi.spyOn(window, "setInterval");
    const clear = vi.spyOn(window, "clearInterval");
    const { result } = renderRecordings(client);
    expect(interval).toHaveBeenCalledOnce();

    act(() => {
      client.setQueryData<TerminalRecordingMap>(key, {});
    });
    expect(clear).toHaveBeenCalledOnce();
    expect(result.current["term-4f21c9a03b7e"]).toBeUndefined();

    act(() => {
      client.setQueryData<TerminalRecordingMap>(key, {
        "term-4f21c9a03b7e": { recordingId: "rec-2", at: AT, profileKey: "work" },
      });
    });
    expect(interval).toHaveBeenCalledTimes(2);
    expect(result.current["term-4f21c9a03b7e"]?.elapsed).toBe("00:00");

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(result.current["term-4f21c9a03b7e"]?.elapsed).toBe("00:01");
  });

  it("Should drop the chip immediately when stop succeeds", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = terminalKeys.recordings(SCOPE);
    client.setQueryData<TerminalRecordingMap>(key, {
      "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    });
    const { result } = renderRecordings(client);
    expect(result.current["term-4f21c9a03b7e"]).toBeDefined();

    act(() => {
      client.setQueryData<TerminalRecordingMap>(key, current =>
        applyRecordingStopSuccess(current ?? {}, {
          terminal_id: "term-4f21c9a03b7e",
          state: "saved",
        })
      );
    });

    expect(result.current["term-4f21c9a03b7e"]).toBeUndefined();
  });

  it("Should leave the chip when stop fails to report a saved recording", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const key = terminalKeys.recordings(SCOPE);
    const live: TerminalRecordingMap = {
      "term-4f21c9a03b7e": { recordingId: "rec-1", at: AT, profileKey: "work" },
    };
    client.setQueryData(key, live);
    const { result } = renderRecordings(client);

    act(() => {
      client.setQueryData<TerminalRecordingMap>(key, current =>
        applyRecordingStopSuccess(current ?? {}, {
          terminal_id: "term-4f21c9a03b7e",
          state: "recording",
        })
      );
    });

    expect(result.current["term-4f21c9a03b7e"]).toBeDefined();
  });
});
