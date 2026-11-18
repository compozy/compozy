import { BrowserWindow } from "electron";

import type { ShellSnapshot } from "../state/app-state";
import { presentWindow, type WindowPresentation } from "./window-presentation";
import type { WindowTheme } from "./window-theme";

/** The boot window: a wide frame so the journey and its progress sit side by side (#1549). */
export const BOOT_WINDOW_SIZE = { width: 880, height: 540 } as const;
const BOOT_WINDOW_MINIMUM = { width: 840, height: 520 } as const;
/** macOS keeps its traffic lights over the page's ink; the page reserves their band. */
const MAC_TRAFFIC_LIGHT_POSITION = { x: 14, y: 14 } as const;

function bootWindowChrome(platform: NodeJS.Platform) {
  return platform === "darwin"
    ? { titleBarStyle: "hidden" as const, trafficLightPosition: MAC_TRAFFIC_LIGHT_POSITION }
    : {};
}

export class BootWindow {
  readonly #window: BrowserWindow;
  readonly #presentation: WindowPresentation;
  #lastSnapshot: ShellSnapshot;

  constructor(options: {
    pagePath: string;
    snapshot?: ShellSnapshot;
    preloadPath: string;
    presentation: WindowPresentation;
    theme: Pick<WindowTheme, "backgroundColor" | "track">;
    onError: (error: Error) => void;
  }) {
    this.#presentation = options.presentation;
    this.#lastSnapshot = options.snapshot ?? { state: "resolving" };
    this.#window = new BrowserWindow({
      width: BOOT_WINDOW_SIZE.width,
      height: BOOT_WINDOW_SIZE.height,
      minWidth: BOOT_WINDOW_MINIMUM.width,
      minHeight: BOOT_WINDOW_MINIMUM.height,
      ...bootWindowChrome(process.platform),
      show: false,
      title: "CompozyOS",
      backgroundColor: options.theme.backgroundColor("canvas"),
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
    options.theme.track(this.#window, "canvas");
    this.#window.webContents.setWindowOpenHandler(() => ({ action: "deny" }));
    this.#window.webContents.on("will-navigate", event => event.preventDefault());
    this.#window.webContents.on("did-finish-load", () => {
      this.render(this.#lastSnapshot);
      presentWindow(this.#window, this.#presentation);
    });
    void this.#window.loadFile(options.pagePath).catch(options.onError);
  }

  render(snapshot: ShellSnapshot): void {
    this.#lastSnapshot = snapshot;
    if (!this.#window.isDestroyed()) this.#window.webContents.send("boot:state", snapshot);
  }

  show(): void {
    if (!this.#window.isDestroyed()) {
      presentWindow(this.#window, this.#presentation);
    }
  }

  /** Whether the window was closed; a closed boot window cannot be shown again. */
  isDestroyed(): boolean {
    return this.#window.isDestroyed();
  }

  close(): void {
    if (!this.#window.isDestroyed()) this.#window.close();
  }
}
