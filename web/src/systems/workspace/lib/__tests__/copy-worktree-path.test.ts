// Suite: worktree path copy behavior
// Invariant: every workspace surface copies the absolute path and reports both
// successful and failed clipboard outcomes through the shared toast contract.
// Owning layer: copyWorktreePath. No existing suite owned this browser boundary.
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { copyWorktreePath } from "../copy-worktree-path";

const toastMocks = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn() }));

vi.mock("sonner", () => ({ toast: toastMocks }));

const clipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");

function installClipboard(writeText: (path: string) => Promise<void>): void {
  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: { writeText },
  });
}

describe("copyWorktreePath", () => {
  beforeEach(() => {
    toastMocks.success.mockClear();
    toastMocks.error.mockClear();
  });

  afterEach(() => {
    if (clipboardDescriptor) {
      Object.defineProperty(navigator, "clipboard", clipboardDescriptor);
    } else {
      Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
    }
  });

  it("Should copy the absolute path and report success", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    installClipboard(writeText);

    copyWorktreePath("/workspace/feature");
    await vi.waitFor(() => expect(toastMocks.success).toHaveBeenCalledWith("Path copied"));

    expect(writeText).toHaveBeenCalledExactlyOnceWith("/workspace/feature");
    expect(toastMocks.error).not.toHaveBeenCalled();
  });

  it("Should report an unavailable clipboard", () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });

    copyWorktreePath("/workspace/feature");

    expect(toastMocks.error).toHaveBeenCalledExactlyOnceWith("Could not copy the path");
  });

  it("Should report an asynchronous clipboard rejection", async () => {
    installClipboard(vi.fn().mockRejectedValue(new Error("permission denied")));

    copyWorktreePath("/workspace/feature");
    await vi.waitFor(() =>
      expect(toastMocks.error).toHaveBeenCalledExactlyOnceWith("Could not copy the path")
    );
  });

  it("Should report a synchronous clipboard failure", () => {
    installClipboard(() => {
      throw new Error("clipboard unavailable");
    });

    copyWorktreePath("/workspace/feature");

    expect(toastMocks.error).toHaveBeenCalledExactlyOnceWith("Could not copy the path");
  });
});
