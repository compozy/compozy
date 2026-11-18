import { basename } from "node:path";
import type { EventEmitter } from "node:events";

import { app, autoUpdater as nativeAutoUpdater } from "electron";
import {
  type AppUpdater,
  autoUpdater,
  CancellationToken,
  type ProgressInfo,
  type UpdateCheckResult,
} from "electron-updater";

import type { AppOperationState } from "./operation-contract";
import { recordedUpdateAsset } from "./electron-installer-policy";
import { recordedReleaseFeed } from "./recorded-release-provider";

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

/** Leaves the recorded-release fallback most of the 60-second download inactivity budget. */
const FEED_CHECK_TIMEOUT_MS = 20_000;

interface ElectronUpdateInstallerOptions {
  readonly updater?: AppUpdater;
  readonly nativeUpdater?: NativeUpdater | null;
  readonly prepareForRestart: (signal: AbortSignal) => Promise<void>;
  /** Reports a channel feed failure that the recorded release download replaced. */
  readonly onFeedFallback?: (error: Error) => void;
}

export class ElectronUpdateInstaller implements AppUpdateInstaller {
  readonly #updater: AppUpdater;
  readonly #nativeUpdater: NativeUpdater | null;
  readonly #prepareForRestart: (signal: AbortSignal) => Promise<void>;
  readonly #onFeedFallback: (error: Error) => void;
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
    this.#onFeedFallback = options.onFeedFallback ?? (() => {});
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
    const { checked, feedError } = await this.#checkRecordedRelease(operation, signal);
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
    } catch (error) {
      signal.throwIfAborted();
      // The recorded provider resolves locally, so an unreachable release fails here.
      throw feedError ? bothSourcesFailed(feedError, error) : error;
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

  /**
   * Reads the channel feed, or the recorded release when the feed host is unreachable.
   *
   * The channel manifest is served from raw.githubusercontent.com, a host the daemon never
   * contacts and that some networks block. The daemon already reached the recorded GitHub
   * release while planning, so that release is the fallback source for the same asset.
   */
  async #checkRecordedRelease(
    operation: AppOperationState,
    signal: AbortSignal
  ): Promise<{ checked: UpdateCheckResult | null; feedError: Error | null }> {
    let feedError: Error;
    try {
      const checked = await this.#checkChannelFeed();
      signal.throwIfAborted();
      return { checked, feedError: null };
    } catch (error) {
      signal.throwIfAborted();
      feedError = asError(error);
    }
    this.#onFeedFallback(feedError);
    try {
      this.#updater.setFeedURL(recordedReleaseFeed(operation));
      const checked = await this.#updater.checkForUpdates();
      signal.throwIfAborted();
      return { checked, feedError };
    } catch (releaseError) {
      signal.throwIfAborted();
      throw bothSourcesFailed(feedError, releaseError);
    } finally {
      // Restores app-update.yml as the feed for the next check; the checked provider stays
      // bound to this download.
      this.#updater.updateConfigPath = null;
    }
  }

  /** Checks the channel feed, failing a stalled request so the fallback can run. */
  async #checkChannelFeed(): Promise<UpdateCheckResult | null> {
    const check = this.#updater.checkForUpdates();
    let timer: NodeJS.Timeout | undefined;
    const expired = new Promise<"expired">(resolve => {
      timer = setTimeout(() => resolve("expired"), FEED_CHECK_TIMEOUT_MS);
      timer.unref();
    });
    try {
      const first = await Promise.race([check, expired]);
      if (first !== "expired") return first;
    } finally {
      clearTimeout(timer);
    }
    // electron-updater cannot cancel one check and hands a pending check to the next caller,
    // so fail the stalled feed request before the fallback checks again.
    await this.#updater.netSession.closeAllConnections();
    try {
      return await check;
    } catch (error) {
      throw new Error("The update feed did not answer in time.", { cause: error });
    }
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

/** Keeps both causes; the release failure stays last because it ended the update. */
function bothSourcesFailed(feedError: Error, releaseError: unknown): AggregateError {
  return new AggregateError(
    [feedError, releaseError],
    "The update feed and the recorded app release could not be reached " +
      `(${feedError.message}; ${asError(releaseError).message}).`
  );
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error));
}
