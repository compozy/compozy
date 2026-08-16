import { BrowserWindow } from "electron";

import type { ShellSnapshot } from "../state/app-state";

export class BootWindow {
  readonly #window: BrowserWindow;
  #lastSnapshot: ShellSnapshot = { state: "resolving" };

  constructor(options: { pagePath: string; preloadPath: string; onError: (error: Error) => void }) {
    this.#window = new BrowserWindow({
      width: 420,
      height: 460,
      minWidth: 380,
      minHeight: 360,
      show: false,
      title: "CompozyOS",
      backgroundColor: "#131211",
      autoHideMenuBar: true,
      webPreferences: {
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
        webviewTag: false,
        devTools: false,
        preload: options.preloadPath,
      },
    });
    this.#window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
    this.#window.webContents.on("will-navigate", event => event.preventDefault());
    this.#window.webContents.on("did-finish-load", () => {
      this.render(this.#lastSnapshot);
      this.#window.show();
      this.#window.focus();
    });
    void this.#window.loadFile(options.pagePath).catch(options.onError);
  }

  render(snapshot: ShellSnapshot): void {
    this.#lastSnapshot = snapshot;
    if (!this.#window.isDestroyed()) this.#window.webContents.send("boot:state", snapshot);
  }

  show(): void {
    if (!this.#window.isDestroyed()) {
      this.#window.show();
      this.#window.focus();
    }
  }

  close(): void {
    if (!this.#window.isDestroyed()) this.#window.close();
  }
}
