import type { BrowserWindow, IpcMain, IpcMainInvokeEvent } from "electron";

import {
  PRODUCT_IPC_CHANNEL,
  isProductMethod,
  validProductParams,
  type GlobalShortcutBinding,
  type OpenInEditorParams,
  type PickDirectoryParams,
  type ThemePreference,
} from "./product-contract";
import type { DirectoryDialog } from "./directory-dialog";
import type { FileOpener } from "./file-opener";
import { GlobalShortcutPolicy } from "../shortcuts/global-shortcut-policy";
import type { WindowTheme } from "../window/window-theme";

/** The product window and the loopback origin it serves, while one is attached. */
export interface ProductTarget {
  window: BrowserWindow;
  origin: string;
}

/**
 * True only for the product window's main frame on the product loopback origin. A subframe,
 * another origin, or another `webContents` (boot window, devtools) never qualifies.
 */
export function isProductMainFrame(
  event: IpcMainInvokeEvent,
  target: ProductTarget | null
): boolean {
  if (!target || target.window.isDestroyed()) return false;
  const frame = event.senderFrame;
  if (!frame || frame !== event.sender.mainFrame) return false;
  if (event.sender !== target.window.webContents) return false;
  let origin: string;
  try {
    origin = new URL(target.origin).origin;
  } catch {
    return false;
  }
  return frame.origin === origin;
}

export class ProductBridgeController {
  readonly #ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
  readonly #shortcuts: GlobalShortcutPolicy;
  readonly #theme: Pick<WindowTheme, "set">;
  readonly #directories: DirectoryDialog;
  readonly #files: FileOpener;
  readonly #productTarget: () => ProductTarget | null;

  constructor(options: {
    ipcMain: Pick<IpcMain, "handle" | "removeHandler">;
    shortcuts: GlobalShortcutPolicy;
    theme: Pick<WindowTheme, "set">;
    directories: DirectoryDialog;
    files: FileOpener;
    productTarget: () => ProductTarget | null;
  }) {
    this.#ipcMain = options.ipcMain;
    this.#shortcuts = options.shortcuts;
    this.#theme = options.theme;
    this.#directories = options.directories;
    this.#files = options.files;
    this.#productTarget = options.productTarget;
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
      if (method === "files.open_in_editor") {
        const target = this.#productTarget();
        if (!isProductMainFrame(event, target)) {
          throw new Error("The product action is not allowed from this frame.");
        }
        return await this.#files.open(params as OpenInEditorParams, target?.window ?? null);
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
