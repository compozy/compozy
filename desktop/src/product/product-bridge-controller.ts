import type { IpcMain, IpcMainInvokeEvent } from "electron";

import {
  PRODUCT_IPC_CHANNEL,
  isProductMethod,
  validProductParams,
  type GlobalShortcutBinding,
  type PickDirectoryParams,
  type ThemePreference,
} from "./product-contract";
import type { DirectoryDialog } from "./directory-dialog";
import { GlobalShortcutPolicy } from "../shortcuts/global-shortcut-policy";
import type { WindowTheme } from "../window/window-theme";

export class ProductBridgeController {
  readonly #ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
  readonly #shortcuts: GlobalShortcutPolicy;
  readonly #theme: Pick<WindowTheme, "set">;
  readonly #directories: DirectoryDialog;

  constructor(options: {
    ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
    shortcuts: GlobalShortcutPolicy;
    theme: Pick<WindowTheme, "set">;
    directories: DirectoryDialog;
  }) {
    this.#ipcMain = options.ipcMain;
    this.#shortcuts = options.shortcuts;
    this.#theme = options.theme;
    this.#directories = options.directories;
  }

  register(): void {
    this.#ipcMain.handle(PRODUCT_IPC_CHANNEL, async (event: IpcMainInvokeEvent, method, params) => {
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
      if (method === "dialog.pick_directory") {
        const { default_path } = params as PickDirectoryParams;
        return { path: await this.#directories.pick(event.sender, default_path) };
      }
      if (method === "global_shortcuts.sync") {
        return this.#shortcuts.sync((params as { bindings: GlobalShortcutBinding[] }).bindings);
      }
      return this.#shortcuts.status();
    });
  }

  unregister(): void {
    this.#ipcMain.removeHandler(PRODUCT_IPC_CHANNEL);
    this.#shortcuts.unregisterAll();
  }
}
