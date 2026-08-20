// Suite: client command channel lifecycle
// Invariant: execute-after-connect reaches the current runner; a stale first
// cleanup must not clear a newer runner; execute before connect rejects.
// Owning layer: web/src/systems/os/lib/client-command-channel.ts
// Canonical suite: this file.
import { describe, expect, it, vi } from "vitest";

import { ClientCommandChannel } from "../client-command-channel";

describe("ClientCommandChannel", () => {
  it("Should execute through the connected runner", async () => {
    const channel = new ClientCommandChannel();
    const runner = vi.fn(async (op: string, payload: unknown) => ({ op, payload }));
    channel.connect(runner);

    await expect(channel.execute("window.close", { id: "w-1" })).resolves.toEqual({
      op: "window.close",
      payload: { id: "w-1" },
    });
    expect(runner).toHaveBeenCalledExactlyOnceWith("window.close", { id: "w-1" });
  });

  it("Should ignore a stale first cleanup after a second connect", async () => {
    const channel = new ClientCommandChannel();
    const first = vi.fn(async () => "first");
    const second = vi.fn(async () => "second");
    const disconnectFirst = channel.connect(first);
    channel.connect(second);
    disconnectFirst();

    await expect(channel.execute("window.focus.last", {})).resolves.toBe("second");
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledOnce();
  });

  it("Should reject execute before connect", async () => {
    const channel = new ClientCommandChannel();
    await expect(channel.execute("window.close", {})).rejects.toThrow(
      "Unsupported client operation: window.close"
    );
  });
});
