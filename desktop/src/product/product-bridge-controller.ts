import type { IpcMain, IpcMainInvokeEvent } from "electron";

import {
  PRODUCT_IPC_CHANNEL,
  isProductMethod,
  validProductParams,
  type GlobalShortcutBinding,
} from "./product-contract";
import { GlobalShortcutPolicy } from "../shortcuts/global-shortcut-policy";

export class ProductBridgeController {
  readonly #ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
  readonly #shortcuts: GlobalShortcutPolicy;

  constructor(options: {
    ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
    shortcuts: GlobalShortcutPolicy;
  }) {
    this.#ipcMain = options.ipcMain;
    this.#shortcuts = options.shortcuts;
  }

  register(): void {
    this.#ipcMain.handle(
      PRODUCT_IPC_CHANNEL,
      async (_event: IpcMainInvokeEvent, method, params) => {
        if (typeof method !== "string" || !isProductMethod(method)) {
          throw new Error("The product action is not supported.");
        }
        if (!validProductParams(method, params)) {
          throw new TypeError("Product action parameters are invalid.");
        }
        if (method === "global_shortcuts.sync") {
          return this.#shortcuts.sync((params as { bindings: GlobalShortcutBinding[] }).bindings);
        }
        return this.#shortcuts.status();
      }
    );
  }

  unregister(): void {
    this.#ipcMain.removeHandler(PRODUCT_IPC_CHANNEL);
    this.#shortcuts.unregisterAll();
  }
}
