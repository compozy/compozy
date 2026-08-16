import { app, BrowserWindow, dialog, screen } from "electron";

import type { DeepLinkQueue } from "../deep-links/deep-link";
import { writeWindowState, readWindowState, type StoredWindowState } from "./window-state-store";
import { CrashRecoveryBudget } from "./crash-recovery-policy";
import { guardWindowNavigation } from "./security";
import { MINIMUM_WINDOW_SIZE, restoreWindowBounds } from "./window-state-policy";
import { nextZoomLevel } from "./zoom-policy";

const REVEAL_DEADLINE_MS = 10_000;
const SAVE_DEBOUNCE_MS = 250;

export class ProductWindow {
  readonly #origin: string;
  readonly #windowStatePath: string;
  readonly #links: DeepLinkQueue;
  readonly #onReady: () => Promise<void>;
  readonly #onLoadFailure: (error: Error) => Promise<void>;
  readonly #onError: (error: Error) => void;
  readonly #crashes = new CrashRecoveryBudget();
  #window: BrowserWindow | null = null;
  #zoomLevel = 0;
  #saveTimer: NodeJS.Timeout | null = null;

  constructor(options: {
    origin: string;
    windowStatePath: string;
    links: DeepLinkQueue;
    onReady: () => Promise<void>;
    onLoadFailure: (error: Error) => Promise<void>;
    onError: (error: Error) => void;
  }) {
    this.#origin = options.origin;
    this.#windowStatePath = options.windowStatePath;
    this.#links = options.links;
    this.#onReady = options.onReady;
    this.#onLoadFailure = options.onLoadFailure;
    this.#onError = options.onError;
  }

  async create(): Promise<BrowserWindow> {
    if (this.#window && !this.#window.isDestroyed()) return this.#window;
    const saved = await readWindowState(this.#windowStatePath);
    const displays = screen.getAllDisplays().map(display => display.workArea);
    const bounds = restoreWindowBounds(saved, displays);
    this.#zoomLevel = saved?.zoom_level ?? 0;
    const window = new BrowserWindow({
      ...bounds,
      minWidth: MINIMUM_WINDOW_SIZE.width,
      minHeight: MINIMUM_WINDOW_SIZE.height,
      show: false,
      title: "CompozyOS",
      backgroundColor: "#131211",
      autoHideMenuBar: true,
      webPreferences: {
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
        webviewTag: false,
        devTools: !app.isPackaged,
      },
    });
    this.#window = window;
    if (saved?.maximized) window.maximize();
    if (!saved) window.center();
    guardWindowNavigation(window, this.#origin, this.#onError);
    this.#wireLifecycle(window);
    const deadline = setTimeout(() => {
      if (!window.isDestroyed() && !window.isVisible()) {
        window.destroy();
        void this.#onLoadFailure(new Error("The product window did not load before its deadline."));
      }
    }, REVEAL_DEADLINE_MS);
    deadline.unref();
    window.webContents.once("did-finish-load", () => clearTimeout(deadline));
    await window.loadURL(this.#origin);
    return window;
  }

  focus(): void {
    const window = this.#window;
    if (!window || window.isDestroyed()) return;
    if (window.isMinimized()) window.restore();
    window.show();
    window.focus();
    this.#navigatePending();
  }

  navigate(path: string): void {
    this.#links.push(`compozyos://open${path}`);
    this.focus();
  }

  adjustZoom(direction: "in" | "out" | "reset"): void {
    const window = this.#window;
    if (!window || window.isDestroyed()) return;
    this.#zoomLevel = nextZoomLevel(this.#zoomLevel, direction);
    window.webContents.setZoomLevel(this.#zoomLevel);
    this.#scheduleSave();
  }

  #wireLifecycle(window: BrowserWindow): void {
    window.webContents.on("did-finish-load", () => {
      window.webContents.setZoomLevel(this.#zoomLevel);
      this.#scheduleSave();
      if (!window.isVisible()) {
        window.show();
        window.focus();
        void this.#onReady().catch(this.#onError);
      }
      this.#navigatePending();
    });
    window.webContents.on("before-input-event", (event, input) => {
      if (!input.control && !input.meta) return;
      const direction =
        input.key === "0"
          ? "reset"
          : input.key === "+" || input.key === "="
            ? "in"
            : input.key === "-"
              ? "out"
              : null;
      if (!direction) return;
      event.preventDefault();
      this.adjustZoom(direction);
    });
    window.on("move", () => this.#scheduleSave());
    window.on("resize", () => this.#scheduleSave());
    window.on("maximize", () => this.#scheduleSave());
    window.on("unmaximize", () => this.#scheduleSave());
    window.webContents.on("render-process-gone", (_event, details) => {
      this.#onError(new Error(`Product renderer exited: ${details.reason}.`));
      if (this.#crashes.record(Date.now()) === "reload") window.reload();
      else
        void dialog.showMessageBox(window, {
          type: "error",
          title: "CompozyOS",
          message: "CompozyOS keeps closing unexpectedly.",
          detail: "Quit and reopen the app. Your sessions remain in the runtime.",
        });
    });
    window.on("unresponsive", () =>
      this.#onError(new Error("Product renderer became unresponsive."))
    );
    window.on("closed", () => {
      this.#links.reset();
      this.#window = null;
    });
  }

  #navigatePending(): void {
    const window = this.#window;
    if (!window || window.isDestroyed()) return;
    const path = this.#links.setReady();
    if (!path) return;
    const target = path === "/" ? this.#origin : new URL(path, this.#origin).toString();
    if (window.webContents.getURL() !== target) void window.loadURL(target).catch(this.#onError);
  }

  #scheduleSave(): void {
    if (this.#saveTimer) clearTimeout(this.#saveTimer);
    this.#saveTimer = setTimeout(() => void this.#save().catch(this.#onError), SAVE_DEBOUNCE_MS);
    this.#saveTimer.unref();
  }

  async #save(): Promise<void> {
    const window = this.#window;
    if (!window || window.isDestroyed()) return;
    const bounds = window.getNormalBounds();
    const state: StoredWindowState = {
      ...bounds,
      maximized: window.isMaximized(),
      zoom_level: this.#zoomLevel,
    };
    await writeWindowState(this.#windowStatePath, state);
  }
}
