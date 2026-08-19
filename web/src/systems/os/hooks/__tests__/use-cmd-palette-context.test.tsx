// Suite: cmd-palette context debouncing
// Invariant: a burst of topology changes settles into at most one availability
// transition, an unattached client yields no snapshot at all, and a real change
// still lands once the burst goes quiet.
// Boundary IN: the debounce window and the attached/unattached gate.
// Boundary OUT: predicate semantics and snapshot field derivation, which the
// evaluator suite owns.
import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resolvePaletteAvailability } from "../../lib/cmd-palette-availability";
import type { CmdPaletteStructuralCommand } from "../../lib/cmd-palette-types";
import { useCmdPaletteContext } from "../use-cmd-palette-context";
import { useDesktop } from "../use-desktop";

vi.mock("../use-desktop", () => ({ useDesktop: vi.fn() }));

const focusedCommand = {
  id: "window.close",
  title: "Close window",
  section: "Window",
  icon: "x-square",
  source: "core",
  bindings: [],
  alias: null,
  destructive: false,
  availability_exempt: false,
  arguments: [],
  action: { kind: "client_op", op: "window.close" },
  execution: { retry_safe: false, single_flight: true },
  when: [{ key: "window.focused", value: true, reason: "requires a focused window" }],
} as unknown as CmdPaletteStructuralCommand;

function topology(focusedId: string | null) {
  return {
    activeDesktopId: "desk-1",
    focusedId,
    frames: {},
    windows:
      focusedId === null
        ? {}
        : { [focusedId]: { id: focusedId, desktopId: "desk-1", minimized: false } },
  };
}

function setFocus(focusedId: string | null): void {
  vi.mocked(useDesktop).mockImplementation((selector: unknown) =>
    (selector as (state: unknown) => unknown)(topology(focusedId))
  );
}

function harness(attached = true) {
  return renderHook(
    (props: { attached: boolean }) =>
      useCmdPaletteContext({
        shellDesktop: false,
        scopeGlobal: false,
        workspaceTrusted: true,
        focusedSessionState: "",
        attached: props.attached,
        debounceMs: 100,
      }),
    { initialProps: { attached } }
  );
}

describe("useCmdPaletteContext (UT-103)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setFocus("win-a");
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("Should render at most one availability transition for a burst of flaps", () => {
    const { result, rerender } = harness();
    const availability = () =>
      resolvePaletteAvailability(focusedCommand, result.current, { daemonReachable: true })
        .available;
    expect(availability()).toBe(true);

    const seen: boolean[] = [availability()];
    // Five flips inside one burst — a reconnect storm.
    for (const focusedId of ["win-a", null, "win-a", null, null]) {
      act(() => {
        setFocus(focusedId);
        rerender({ attached: true });
        vi.advanceTimersByTime(20);
      });
      seen.push(availability());
    }
    act(() => {
      vi.advanceTimersByTime(200);
    });
    seen.push(availability());

    const transitions = seen.filter((value, index) => index > 0 && value !== seen[index - 1]);
    expect(transitions).toHaveLength(1);
    // …and it settles on the truth the burst ended at.
    expect(availability()).toBe(false);
  });

  it("Should settle on the quiet value once the burst ends", () => {
    const { result, rerender } = harness();
    expect(result.current?.["window.focused"]).toBe(true);
    // Commit first, so the debounce effect has scheduled before the clock moves.
    act(() => {
      setFocus(null);
      rerender({ attached: true });
    });
    expect(result.current?.["window.focused"]).toBe(true);
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current?.["window.focused"]).toBe(false);
  });

  it("Should produce no snapshot at all while no client is attached", () => {
    const { result } = harness(false);
    expect(result.current).toBeNull();
    // The evaluator turns that into a refusal, never an allow-all.
    expect(
      resolvePaletteAvailability(focusedCommand, result.current, { daemonReachable: true })
    ).toEqual({
      visible: true,
      available: false,
      reason: "requires an attached shell",
    });
  });
});
