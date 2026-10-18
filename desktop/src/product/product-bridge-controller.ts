import type { IpcMain, IpcMainInvokeEvent } from "electron";

import {
  PRODUCT_IPC_CHANNEL,
  isProductMethod,
  validProductParams,
  type GlobalShortcutBinding,
  type ThemePreference,
} from "./product-contract";
import { GlobalShortcutPolicy } from "../shortcuts/global-shortcut-policy";
import type { WindowTheme } from "../window/window-theme";

export class ProductBridgeController {
  readonly #ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
  readonly #shortcuts: GlobalShortcutPolicy;
  readonly #theme: Pick<WindowTheme, "set">;

  constructor(options: {
    ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
    shortcuts: GlobalShortcutPolicy;
    theme: Pick<WindowTheme, "set">;
  }) {
    this.#ipcMain = options.ipcMain;
    this.#shortcuts = options.shortcuts;
    this.#theme = options.theme;
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
        if (method === "theme.set") {
          const { preference } = params as { preference: ThemePreference };
          await this.#theme.set(preference);
          return { preference };
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
