import { basename } from "node:path";

import { type AppUpdater, autoUpdater, type ProgressInfo } from "electron-updater";

import type { AppOperationState } from "./operation-contract";

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

function normalizedVersion(version: string): string {
  return version.trim().replace(/^v/, "");
}

export class ElectronUpdateInstaller implements AppUpdateInstaller {
  readonly #updater: AppUpdater;

  constructor(updater: AppUpdater = autoUpdater) {
    this.#updater = updater;
    this.#updater.autoDownload = false;
    this.#updater.autoInstallOnAppQuit = false;
  }

  async download(
    operation: AppOperationState,
    onProgress: (percent: number) => Promise<void>
  ): Promise<DownloadedAppUpdate> {
    const checked = await this.#updater.checkForUpdates();
    if (
      !checked ||
      normalizedVersion(checked.updateInfo.version) !== normalizedVersion(operation.to_version)
    ) {
      throw new Error("The update feed does not match the recorded app release.");
    }
    const recordedAsset = operation.asset.trim();
    const feedContainsAsset = checked.updateInfo.files.some(
      file => basename(file.url) === recordedAsset
    );
    if (!feedContainsAsset) {
      throw new Error("The update feed does not contain the recorded app asset.");
    }

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
