// Invariant: leaving a main session marks it seen whether or not a digest showed, so reports the
// operator already read never come back as "While you were away".
// Owner: this hook; the window view only says whether a main session is on screen.
import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { markAgentInstanceSeen } from "@/systems/agent";

import { useMarkSeenOnLeave } from "../use-mark-seen-on-leave";

vi.mock("@/systems/agent", () => ({ markAgentInstanceSeen: vi.fn(async () => undefined) }));

const INSTANCE = { id: "ainst-1", profile_name: "work" };

describe("useMarkSeenOnLeave", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.mocked(markAgentInstanceSeen).mockClear();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("Should mark the instance seen in its own profile on leaving", () => {
    const { unmount } = renderHook(() => useMarkSeenOnLeave(INSTANCE, true));
    vi.advanceTimersByTime(1_500);
    unmount();
    expect(markAgentInstanceSeen).toHaveBeenCalledExactlyOnceWith(INSTANCE);
  });

  it("Should not count a remount in the same tick as leaving", () => {
    const { unmount } = renderHook(() => useMarkSeenOnLeave(INSTANCE, true));
    unmount();
    expect(markAgentInstanceSeen).not.toHaveBeenCalled();
  });

  it("Should do nothing for a session that isn't a main session", () => {
    const { unmount } = renderHook(() => useMarkSeenOnLeave(INSTANCE, false));
    vi.advanceTimersByTime(1_500);
    unmount();
    expect(markAgentInstanceSeen).not.toHaveBeenCalled();
  });
});
