import { describe, expect, it, vi } from "vitest";
import type { BrowserWindow, IpcMainInvokeEvent } from "electron";

import type { FileOpener } from "../file-opener";
import { ProductBridgeController, type ProductTarget } from "../product-bridge-controller";
import { PRODUCT_IPC_CHANNEL } from "../product-contract";
import type { GlobalShortcutPolicy } from "../../shortcuts/global-shortcut-policy";

const ORIGIN = "http://127.0.0.1:2123";
const PARAMS = { path: "/repo/a.ts", line: 3, roots: ["/repo"] };

type Handler = (event: IpcMainInvokeEvent, method: unknown, params: unknown) => Promise<unknown>;

function frame(origin: string) {
  return { origin, url: `${origin}/sessions` };
}

function harness() {
  let handler: Handler | null = null;
  const mainFrame = frame(ORIGIN);
  const productContents = { mainFrame };
  const window = {
    webContents: productContents,
    isDestroyed: () => false,
  } as unknown as BrowserWindow;
  let target: ProductTarget | null = { window, origin: ORIGIN };
  const open = vi.fn<FileOpener["open"]>(async () => ({ status: "opened" }));
  const controller = new ProductBridgeController({
    ipcMain: {
      handle: (channel: string, listener: Handler) => {
        expect(channel).toBe(PRODUCT_IPC_CHANNEL);
        handler = listener;
      },
      removeHandler: vi.fn(),
    } as never,
    shortcuts: {} as GlobalShortcutPolicy,
    theme: { set: vi.fn() },
    directories: { pick: vi.fn() },
    files: { open },
    productTarget: () => target,
  });
  controller.register();
  const invoke = async (event: Partial<IpcMainInvokeEvent>, params: unknown = PARAMS) =>
    await handler!(event as IpcMainInvokeEvent, "files.open_in_editor", params);
  return {
    invoke,
    open,
    window,
    productEvent: { sender: productContents, senderFrame: mainFrame } as never,
    productContents,
    mainFrame,
    detach: () => {
      target = null;
    },
  };
}

describe("product bridge controller", () => {
  it("Should open a file only for the product window main frame on the loopback origin", async () => {
    const run = harness();
    expect(await run.invoke(run.productEvent)).toEqual({ status: "opened" });
    expect(run.open).toHaveBeenCalledWith(PARAMS, run.window);
  });

  it("Should reject another origin, a subframe, another webContents, or no product window", async () => {
    const run = harness();
    const foreignMain = frame("http://127.0.0.1:9999");
    const rejected: Partial<IpcMainInvokeEvent>[] = [
      { sender: { mainFrame: foreignMain } as never, senderFrame: foreignMain as never },
      { sender: run.productContents as never, senderFrame: frame(ORIGIN) as never },
      { sender: run.productContents as never, senderFrame: null },
      (() => {
        const other = frame(ORIGIN);
        return { sender: { mainFrame: other } as never, senderFrame: other as never };
      })(),
    ];
    for (const event of rejected) {
      await expect(run.invoke(event)).rejects.toThrow("not allowed from this frame");
    }
    run.mainFrame.origin = "http://127.0.0.1:9999";
    await expect(run.invoke(run.productEvent)).rejects.toThrow("not allowed from this frame");
    run.mainFrame.origin = ORIGIN;
    run.detach();
    await expect(run.invoke(run.productEvent)).rejects.toThrow("not allowed from this frame");
    expect(run.open).not.toHaveBeenCalled();
  });

  it("Should reject invalid parameters before the sender or opener is consulted", async () => {
    const run = harness();
    await expect(
      run.invoke(run.productEvent, { ...PARAMS, roots: ["//host/share"] })
    ).rejects.toThrow("parameters are invalid");
    expect(run.open).not.toHaveBeenCalled();
  });
});
