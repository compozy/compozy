// Suite: OS session-list lifecycle host
// Invariant: an archive or delete a list confirms retires every window showing that session,
// and only those; Move to Later leaves windows alone.
// Boundary IN: the mapping from confirmed lifecycle outcomes to window retirement.
// Boundary OUT: the mutations (use-session-actions) and retirement itself (routing coordinator).
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { useSessionLifecycleActions } from "@/systems/session";

import { useSessionListLifecycleActions } from "../use-session-list-lifecycle-actions";

type LifecycleOptions = NonNullable<Parameters<typeof useSessionLifecycleActions>[0]>;

const harness = vi.hoisted(() => ({
  options: {} as LifecycleOptions,
  userRetireSession: vi.fn((_windowId: string) => Promise.resolve(true)),
  windows: {
    "session:open": { id: "session:open", app: "session", instanceKey: "sess-open" },
    "session:twin": { id: "session:twin", app: "session", instanceKey: "sess-open" },
    "session:other": { id: "session:other", app: "session", instanceKey: "sess-other" },
    "agents:open": { id: "agents:open", app: "agents", instanceKey: "sess-open" },
  },
}));

vi.mock("@/systems/session", () => ({
  useSessionLifecycleActions: (options: LifecycleOptions) => {
    harness.options = options;
    return {};
  },
}));

vi.mock("../use-os-shell", () => ({
  useOsShell: () => ({
    coordinator: { userRetireSession: harness.userRetireSession },
    manager: { getState: () => ({ windows: harness.windows }) },
  }),
}));

const openSession = { id: "sess-open" } as Parameters<
  NonNullable<LifecycleOptions["onArchiveSuccess"]>
>[0];

describe("useSessionListLifecycleActions", () => {
  beforeEach(() => harness.userRetireSession.mockClear());

  it.each([
    ["archive", () => harness.options.onArchiveSuccess?.(openSession)],
    ["delete", () => harness.options.onDeleteSuccess?.(openSession)],
  ])("Should retire every window showing a session after %s", async (action, confirm) => {
    const onRemoved = vi.fn();
    renderHook(() => useSessionListLifecycleActions({ onRemoved }));

    await confirm();

    expect(onRemoved).toHaveBeenCalledExactlyOnceWith(openSession, action);
    expect(harness.userRetireSession.mock.calls.map(([windowId]) => windowId)).toEqual([
      "session:open",
      "session:twin",
    ]);
  });

  it("Should leave windows alone after Move to Later", () => {
    const onRemoved = vi.fn();
    renderHook(() => useSessionListLifecycleActions({ onRemoved }));

    harness.options.onLaterSuccess?.(openSession);

    expect(onRemoved).toHaveBeenCalledExactlyOnceWith(openSession, "later");
    expect(harness.userRetireSession).not.toHaveBeenCalled();
  });
});
