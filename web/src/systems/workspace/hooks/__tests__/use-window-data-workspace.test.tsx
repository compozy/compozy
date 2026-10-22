// Suite: window data binding
// Invariant: one precedence rule for every window — a serialized binding wins in every
// partition (`"<id>"` → that project, `"@user"` → User); an unbound window reads its partition
// (`global` → the aggregate, a project → that project, `user` → User). Outside a window the
// hook reads the menubar destination.
// Owning layer: workspace hook. Canonical suite: this file (state table W-05).
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../use-active-workspace", () => ({
  useDataScope: () => ({ kind: "project", workspaceId: "ws_focus" }),
}));

import { windowDataScope } from "../../lib/window-data-scope";
import { useWindowDataWorkspace, WindowDataBindingContext } from "../use-window-data-workspace";

describe("windowDataScope", () => {
  it.each([
    ["global", undefined, { kind: "all" }],
    ["global", "ws_b", { kind: "project", workspaceId: "ws_b" }],
    ["global", "@user", { kind: "user" }],
    ["ws_a", undefined, { kind: "project", workspaceId: "ws_a" }],
    ["ws_a", "ws_q", { kind: "project", workspaceId: "ws_q" }],
    ["ws_a", "@user", { kind: "user" }],
    ["user", undefined, { kind: "user" }],
    ["user", "ws_b", { kind: "project", workspaceId: "ws_b" }],
  ] as const)("Should read partition %s with binding %s", (partition, binding, expected) => {
    expect(windowDataScope(partition, binding)).toEqual(expected);
  });

  it("Should claim nothing while the partition is unknown and the window is unbound", () => {
    expect(windowDataScope(null, undefined)).toBeNull();
    expect(windowDataScope("global", "")).toEqual({ kind: "all" });
  });
});

describe("useWindowDataWorkspace", () => {
  it("Should read the window binding inside a window", () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <WindowDataBindingContext value={{ partition: "global", binding: "ws_b" }}>
        {children}
      </WindowDataBindingContext>
    );
    const { result } = renderHook(() => useWindowDataWorkspace(), { wrapper });

    expect(result.current).toEqual({ kind: "project", workspaceId: "ws_b" });
  });

  it("Should read the menubar destination outside a window", () => {
    const { result } = renderHook(() => useWindowDataWorkspace());

    expect(result.current).toEqual({ kind: "project", workspaceId: "ws_focus" });
  });
});
