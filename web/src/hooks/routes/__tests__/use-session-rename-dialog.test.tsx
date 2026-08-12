import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { useSessionRenameDialog } from "../use-session-rename-dialog";

function createDeferredPromise<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  const promise = new Promise<T>(res => {
    resolve = res;
  });
  return { promise, resolve };
}

describe("useSessionRenameDialog", () => {
  it("Should keep the dialog open and expose the rename failure", async () => {
    const onRename = vi.fn().mockRejectedValue(new Error("Rename unavailable."));
    const { result } = renderHook(() => useSessionRenameDialog(onRename));

    act(() => result.current.openDialog());
    act(() => result.current.confirmRename("Release review"));

    await waitFor(() => expect(result.current.error).toBe("Rename unavailable."));
    expect(result.current.open).toBe(true);
  });

  it("Should close the dialog only after a successful rename", async () => {
    const rename = createDeferredPromise<void>();
    const { result } = renderHook(() => useSessionRenameDialog(() => rename.promise));

    act(() => result.current.openDialog());
    act(() => result.current.confirmRename("Release review"));
    expect(result.current.open).toBe(true);

    await act(async () => {
      rename.resolve();
      await rename.promise;
    });
    expect(result.current.open).toBe(false);
    expect(result.current.error).toBeNull();
  });
});
