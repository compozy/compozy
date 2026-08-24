// Suite: profile event-stream hook lifecycle
// Invariant: reconnecting starts stale and becomes live only after the new source opens.
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
});
