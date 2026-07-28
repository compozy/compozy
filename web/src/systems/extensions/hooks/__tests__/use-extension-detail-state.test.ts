// Suite: Extension detail dialog state
// Invariant: extension detail can select at most one dialog at a time.
// Boundary IN: useExtensionDetailState dialog actions.
// Boundary OUT: Extension queries, mutations, and rendered dialog primitives.

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  bundles: { data: [] },
  detail: { data: null },
  navigate: vi.fn(),
  toggle: { mutate: vi.fn() },
  update: { mutate: vi.fn() },
}));

vi.mock("@tanstack/react-router", () => ({ useNavigate: () => mocks.navigate }));
vi.mock("../use-extension-actions", () => ({
  useToggleExtension: () => mocks.toggle,
  useUpdateExtension: () => mocks.update,
}));
vi.mock("../use-extensions", () => ({
  useBundleActivations: () => mocks.bundles,
  useExtensionDetail: () => mocks.detail,
}));

import { useExtensionDetailState } from "../use-extension-detail-state";

describe("useExtensionDetailState", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("Should replace provenance with removal before a dialog is dismissed", () => {
    const { result } = renderHook(() => useExtensionDetailState("otel-bridge"));

    act(() => {
      result.current.requestProvenance();
    });
    expect(result.current.activeDialog).toBe("provenance");

    act(() => {
      result.current.requestRemoval();
    });
    expect(result.current.activeDialog).toBe("remove");

    act(() => {
      result.current.dismissDialog();
    });
    expect(result.current.activeDialog).toBeNull();
  });
});
