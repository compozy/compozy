// @vitest-environment jsdom
// Suite: network elapsed clock
// Invariant: elapsed labels tick only while their owning Network window is live.
// Boundary IN: second-clock subscription, liveness, and elapsed formatting.
// Boundary OUT: Network window visibility derivation, owned by OS hook suites.

import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { NetworkLiveDataContext } from "../../contexts/network-live-data-context";
import { formatElapsedSeconds, useElapsedSeconds } from "../use-elapsed";

afterEach(() => {
  vi.useRealTimers();
});

describe("formatElapsedSeconds", () => {
  it("returns empty string for null", () => {
    expect(formatElapsedSeconds(null)).toBe("");
  });

  it("formats seconds under a minute", () => {
    expect(formatElapsedSeconds(0)).toBe("0s");
    expect(formatElapsedSeconds(45)).toBe("45s");
  });

  it("formats minutes and remainder", () => {
    expect(formatElapsedSeconds(60)).toBe("1m");
    expect(formatElapsedSeconds(125)).toBe("2m 5s");
  });

  it("formats hours and remainder", () => {
    expect(formatElapsedSeconds(3_600)).toBe("1h");
    expect(formatElapsedSeconds(3_660)).toBe("1h 1m");
  });
});

describe("useElapsedSeconds", () => {
  it("returns null when start is null", () => {
    const { result } = renderHook(() => useElapsedSeconds(null));
    expect(result.current).toBeNull();
  });

  it("returns null when start is invalid", () => {
    const { result } = renderHook(() => useElapsedSeconds("not-a-date"));
    expect(result.current).toBeNull();
  });

  it("returns a non-negative second count for a recent timestamp", () => {
    const { result } = renderHook(() => useElapsedSeconds(new Date()));
    expect(result.current).toBeGreaterThanOrEqual(0);
  });

  it("does not subscribe to the shared clock when its retained network window is inactive", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(0));
    const wrapper = ({ children }: { children: ReactNode }) =>
      createElement(NetworkLiveDataContext, { value: false }, children);

    const { result } = renderHook(() => useElapsedSeconds(new Date(0)), { wrapper });

    const initialElapsed = result.current;
    act(() => vi.advanceTimersByTime(1_000));
    expect(result.current).toBe(initialElapsed);
  });
});
