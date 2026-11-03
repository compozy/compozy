// Suite: profile event-stream hook lifecycle
// Invariant: reconnecting starts stale and becomes live only after the new source opens; only a
// rename event, or a reconnect that may have missed one, reaches the caller's rename handler, for
// caches outside the profiles system (#1220).
// Owning layer: web/src/systems/profiles/hooks/use-profile-event-stream.ts.
// Boundary OUT: event parsing and cache invalidation, owned by profile-event-stream.test.ts.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { describe, expect, it, vi } from "vitest";

import { useProfileEventStream } from "../use-profile-event-stream";

const queryClient = new QueryClient();

function wrapper({ children }: PropsWithChildren) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

describe("useProfileEventStream", () => {
  it("Should reset connection health while a disabled stream reconnects", () => {
    const sources: Array<Map<string, EventListener>> = [];
    const eventSourceFactory = vi.fn(() => {
      const listeners = new Map<string, EventListener>();
      sources.push(listeners);
      return {
        addEventListener: (name: string, listener: EventListener) => listeners.set(name, listener),
        removeEventListener: (name: string) => listeners.delete(name),
        close: vi.fn(),
      } as never;
    });
    const { result, rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) => useProfileEventStream({ enabled, eventSourceFactory }),
      { initialProps: { enabled: true }, wrapper }
    );

    expect(result.current).toBe("stale");
    act(() => sources[0]?.get("open")?.(new Event("open")));
    expect(result.current).toBe("live");

    rerender({ enabled: false });
    expect(result.current).toBe("disabled");
    rerender({ enabled: true });

    expect(result.current).toBe("stale");
    expect(eventSourceFactory).toHaveBeenCalledTimes(2);
  });

  it("Should hand only rename events to the caller's rename handler", () => {
    const listeners = new Map<string, EventListener>();
    const eventSourceFactory = () =>
      ({
        addEventListener: (name: string, listener: EventListener) => listeners.set(name, listener),
        removeEventListener: (name: string) => listeners.delete(name),
        close: vi.fn(),
      }) as never;
    const onProfileRenamed = vi.fn();
    renderHook(() => useProfileEventStream({ eventSourceFactory, onProfileRenamed }), { wrapper });
    const frame = (name: string, content: Record<string, string>) =>
      new MessageEvent(name, {
        data: JSON.stringify({ type: name, content: { name, ...content } }),
      });

    act(() => listeners.get("profile.created")?.(frame("profile.created", { profile_name: "a" })));
    expect(onProfileRenamed).not.toHaveBeenCalled();

    act(() =>
      listeners.get("profile.renamed")?.(
        frame("profile.renamed", { profile_name: "b", previous_profile_name: "a" })
      )
    );
    expect(onProfileRenamed).toHaveBeenCalledTimes(1);
  });

  it("Should hand a reconnect, but not the first open, to the rename handler", () => {
    const sources: Array<Map<string, EventListener>> = [];
    const eventSourceFactory = () => {
      const listeners = new Map<string, EventListener>();
      sources.push(listeners);
      return {
        addEventListener: (name: string, listener: EventListener) => listeners.set(name, listener),
        removeEventListener: (name: string) => listeners.delete(name),
        close: vi.fn(),
      } as never;
    };
    const onProfileRenamed = vi.fn();
    const { rerender } = renderHook(
      ({ enabled }: { enabled: boolean }) =>
        useProfileEventStream({ enabled, eventSourceFactory, onProfileRenamed }),
      { initialProps: { enabled: true }, wrapper }
    );

    act(() => sources[0]?.get("open")?.(new Event("open")));
    expect(onProfileRenamed).not.toHaveBeenCalled();

    // A rename while the socket is down is not replayed, so the reconnect must reread.
    rerender({ enabled: false });
    rerender({ enabled: true });
    act(() => sources[1]?.get("open")?.(new Event("open")));
    expect(onProfileRenamed).toHaveBeenCalledTimes(1);
  });
});
