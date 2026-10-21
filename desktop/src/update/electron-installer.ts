import { basename } from "node:path";
import type { EventEmitter } from "node:events";

import { app, autoUpdater as nativeAutoUpdater } from "electron";
import {
  type AppUpdater,
  autoUpdater,
  CancellationToken,
  type ProgressInfo,
} from "electron-updater";

import type { AppOperationState } from "./operation-contract";
import { recordedUpdateAsset } from "./electron-installer-policy";

export interface DownloadedAppUpdate {
  readonly artifactPath: string;
  readonly version: string;
}

export interface AppUpdateInstaller {
  download(
    operation: AppOperationState,
    onProgress: (percent: number) => Promise<void>,
    signal: AbortSignal
  ): Promise<DownloadedAppUpdate>;
  prepareInstall(signal: AbortSignal): Promise<void>;
  quitAndInstall(signal: AbortSignal): Promise<void>;
}

type NativeUpdater = Pick<EventEmitter, "on" | "off"> & { checkForUpdates(): void };

interface ElectronUpdateInstallerOptions {
  readonly updater?: AppUpdater;
  readonly nativeUpdater?: NativeUpdater | null;
  readonly prepareForRestart: (signal: AbortSignal) => Promise<void>;
}

export class ElectronUpdateInstaller implements AppUpdateInstaller {
  readonly #updater: AppUpdater;
  readonly #nativeUpdater: NativeUpdater | null;
  readonly #prepareForRestart: (signal: AbortSignal) => Promise<void>;
  #prepared = false;

  constructor(options: ElectronUpdateInstallerOptions) {
    this.#updater = options.updater ?? autoUpdater;
    this.#nativeUpdater =
      options.nativeUpdater === undefined
        ? process.platform === "darwin"
          ? nativeAutoUpdater
          : null
        : options.nativeUpdater;
    this.#prepareForRestart = options.prepareForRestart;
    this.#updater.autoDownload = false;
    this.#updater.autoInstallOnAppQuit = false;
    this.#updater.disableDifferentialDownload = app.runningUnderARM64Translation;
  }

  async download(
    operation: AppOperationState,
    onProgress: (percent: number) => Promise<void>,
    signal: AbortSignal
  ): Promise<DownloadedAppUpdate> {
    signal.throwIfAborted();
    this.#prepared = false;
    const checked = await this.#updater.checkForUpdates();
    signal.throwIfAborted();
    if (!checked?.isUpdateAvailable) {
      throw new Error("The update feed does not match the recorded app release.");
    }
    const recordedAsset = recordedUpdateAsset(checked.updateInfo, operation);

    let progressQueue = Promise.resolve();
    let progressError: unknown;
    const cancellation = new CancellationToken();
    const cancel = () => cancellation.cancel();
    signal.addEventListener("abort", cancel, { once: true });
    const handleProgress = (progress: ProgressInfo): void => {
      if (signal.aborted) return;
      const percent = Math.max(0, Math.min(100, Math.round(progress.percent)));
      progressQueue = progressQueue
        .then(async () => {
          signal.throwIfAborted();
          await onProgress(percent);
        })
        .catch(error => {
          progressError = error;
        });
    };
    this.#updater.on("download-progress", handleProgress);
    let paths: string[];
    try {
      signal.throwIfAborted();
      paths = await this.#updater.downloadUpdate(cancellation);
      signal.throwIfAborted();
      await progressQueue;
    } finally {
      signal.removeEventListener("abort", cancel);
      this.#updater.off("download-progress", handleProgress);
    }
    if (progressError) throw progressError;
    const artifactPath = paths.find(path => basename(path) === recordedAsset);
    if (!artifactPath) {
      throw new Error("The downloaded update does not match the recorded app asset.");
    }
    return { artifactPath, version: checked.updateInfo.version };
  }

  async prepareInstall(signal: AbortSignal): Promise<void> {
    signal.throwIfAborted();
    this.#prepared = false;
    const nativeUpdater = this.#nativeUpdater;
    if (nativeUpdater) {
      // With autoInstallOnAppQuit disabled, electron-updater only prepares its ZIP proxy.
      // Squirrel must finish staging before any shell resource is closed.
      await new Promise<void>((resolve, reject) => {
        const clear = () => {
          nativeUpdater.off("update-downloaded", ready);
          nativeUpdater.off("error", failed);
          signal.removeEventListener("abort", canceled);
        };
        const ready = () => {
          clear();
          resolve();
        };
        const failed = (error: Error) => {
          clear();
          reject(error);
        };
        const canceled = () => {
          clear();
          reject(signal.reason);
        };
        nativeUpdater.on("update-downloaded", ready);
        nativeUpdater.on("error", failed);
        signal.addEventListener("abort", canceled, { once: true });
        try {
          nativeUpdater.checkForUpdates();
        } catch (error) {
          clear();
          reject(error);
        }
      });
    }
    signal.throwIfAborted();
    this.#prepared = true;
  }

  async quitAndInstall(signal: AbortSignal): Promise<void> {
    signal.throwIfAborted();
    if (!this.#prepared) throw new Error("The app installer is not ready to restart.");
    await this.#prepareForRestart(signal);
    signal.throwIfAborted();
    this.#updater.quitAndInstall(false, true);
  }
}
