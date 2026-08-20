import type { IpcMain, IpcMainInvokeEvent } from "electron";

import {
  PRODUCT_METHODS,
  validProductParams,
  type GlobalShortcutBinding,
  type ProductMethod,
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
    this.#ipcMain.handle("product:control", async (_event: IpcMainInvokeEvent, method, params) => {
      if (typeof method !== "string" || !PRODUCT_METHODS.has(method)) {
        throw new Error("The product action is not supported.");
      }
      if (!validProductParams(method as ProductMethod, params)) {
        throw new TypeError("Product action parameters are invalid.");
      }
      if (method === "global_shortcuts.sync") {
        return this.#shortcuts.sync((params as { bindings: GlobalShortcutBinding[] }).bindings);
      }
      return this.#shortcuts.status();
    });
  }

  unregister(): void {
    this.#ipcMain.removeHandler("product:control");
    this.#shortcuts.unregisterAll();
  }
}
