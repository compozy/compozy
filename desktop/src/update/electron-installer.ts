import { basename } from "node:path";
import { arch as machineArchitecture } from "node:os";

import { type AppUpdater, autoUpdater, type ProgressInfo } from "electron-updater";

import type { AppOperationState } from "./operation-contract";
import { shouldDisableDifferentialDownload } from "../release/cross-arch";
import { recordedUpdateAsset } from "./electron-installer-policy";

export interface DownloadedAppUpdate {
  readonly artifactPath: string;
  readonly version: string;
}

export interface AppUpdateInstaller {
  download(
    operation: AppOperationState,
    onProgress: (percent: number) => Promise<void>
  ): Promise<DownloadedAppUpdate>;
  quitAndInstall(): void;
}

export class ElectronUpdateInstaller implements AppUpdateInstaller {
  readonly #updater: AppUpdater;

  constructor(updater: AppUpdater = autoUpdater) {
    this.#updater = updater;
    this.#updater.autoDownload = false;
    this.#updater.autoInstallOnAppQuit = false;
    this.#updater.disableDifferentialDownload = shouldDisableDifferentialDownload(
      process.arch,
      machineArchitecture()
    );
  }

  async download(
    operation: AppOperationState,
    onProgress: (percent: number) => Promise<void>
  ): Promise<DownloadedAppUpdate> {
    const checked = await this.#updater.checkForUpdates();
    if (!checked) throw new Error("The update feed does not match the recorded app release.");
    const recordedAsset = recordedUpdateAsset(checked.updateInfo, operation);

    let progressQueue = Promise.resolve();
    let progressError: unknown;
    const handleProgress = (progress: ProgressInfo): void => {
      const percent = Math.max(0, Math.min(100, Math.round(progress.percent)));
      progressQueue = progressQueue
        .then(async () => await onProgress(percent))
        .catch(error => {
          progressError = error;
        });
    };
    this.#updater.on("download-progress", handleProgress);
    let paths: string[];
    try {
      paths = await this.#updater.downloadUpdate();
      await progressQueue;
    } finally {
      this.#updater.off("download-progress", handleProgress);
    }
    if (progressError) throw progressError;
    const artifactPath = paths.find(path => basename(path) === recordedAsset);
    if (!artifactPath) {
      throw new Error("The downloaded update does not match the recorded app asset.");
    }
    return { artifactPath, version: checked.updateInfo.version };
  }

  quitAndInstall(): void {
    this.#updater.quitAndInstall(false, true);
  }
}
