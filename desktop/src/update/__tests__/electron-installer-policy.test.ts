import { EventEmitter } from "node:events";

import { describe, expect, it, vi } from "vitest";
import type {
  AppUpdater,
  CancellationToken,
  ResolvedUpdateFileInfo,
  UpdateCheckResult,
} from "electron-updater";
import type { ProviderRuntimeOptions } from "electron-updater/out/providers/Provider";

import { ElectronUpdateInstaller } from "../electron-installer";
import { recordedUpdateAsset } from "../electron-installer-policy";
import type { AppOperationState } from "../operation-contract";
import type { RecordedReleaseFeed } from "../recorded-release-provider";

const architecture = vi.hoisted(() => ({ translated: false }));
vi.mock("electron", () => ({
  autoUpdater: {},
  app: {
    get runningUnderARM64Translation() {
      return architecture.translated;
    },
  },
}));
vi.mock("electron-updater", async () => {
  const { CancellationToken } = await import("electron-updater/out/types");
  const { Provider } = await import("electron-updater/out/providers/Provider");
  return { autoUpdater: {}, CancellationToken, Provider };
});

const operation: AppOperationState = {
  from_version: "1.0.0",
  to_version: "1.1.0",
  release_tag: "v1.1.0",
  asset: "CompozyOS-1.1.0-arm64.zip",
  digest: "sha256:abc",
  attempt_id: "attempt-1",
  phase: "staged",
  consecutive_failures: 0,
};

/** Stands in for electron-updater's feed selection: app-update.yml unless setFeedURL overrides it. */
class FeedUpdater extends EventEmitter {
  feed: RecordedReleaseFeed | null = null;
  resolved: ResolvedUpdateFileInfo[] = [];
  #checkedRelease = false;
  readonly #failures: { readonly channel: Error; readonly release?: Error };
  readonly #files: readonly string[];

  constructor(
    failures: { readonly channel: Error; readonly release?: Error },
    files: readonly string[]
  ) {
    super();
    this.#failures = failures;
    this.#files = files;
  }

  set updateConfigPath(_path: string | null) {
    this.feed = null;
  }

  setFeedURL(feed: RecordedReleaseFeed): void {
    this.feed = feed;
  }

  async checkForUpdates(): Promise<UpdateCheckResult> {
    if (!this.feed) throw this.#failures.channel;
    this.#checkedRelease = true;
    const runtime: ProviderRuntimeOptions = {
      isUseMultipleRangeRequest: false,
      platform: "darwin",
      executor: {} as ProviderRuntimeOptions["executor"],
    };
    const provider = new this.feed.updateProvider({}, this, runtime);
    const updateInfo = await provider.getLatestVersion();
    this.resolved = provider.resolveFiles(updateInfo);
    return { isUpdateAvailable: true, updateInfo, versionInfo: updateInfo };
  }

  async downloadUpdate(): Promise<string[]> {
    // The recorded provider resolves locally; its release host is first contacted here.
    if (this.#checkedRelease && this.#failures.release) throw this.#failures.release;
    return [...this.#files];
  }
}

// Invariant: installer feed identity must exactly match the durable app operation.
describe("electron installer feed policy", () => {
  // Invariant: a Rosetta-to-native update never applies an x64 differential to an arm64 artifact.
  // Owner: Electron installer adapter. Canonical suite: electron-installer-policy.test.ts.
  it.each([true, false])("Should use Electron's Rosetta detection (%s)", translated => {
    architecture.translated = translated;
    try {
      const updater = {} as AppUpdater;
      new ElectronUpdateInstaller({
        updater,
        nativeUpdater: null,
        prepareForRestart: async () => {},
      });
      expect(updater.disableDifferentialDownload).toBe(translated);
    } finally {
      architecture.translated = false;
    }
  });

  it("Should accept the recorded version and asset", () => {
    expect(
      recordedUpdateAsset({ version: "v1.1.0", files: [{ url: operation.asset }] }, operation)
    ).toBe(operation.asset);
  });

  it("Should reject a different version or asset", () => {
    expect(() =>
      recordedUpdateAsset({ version: "1.2.0", files: [{ url: operation.asset }] }, operation)
    ).toThrow("does not match the recorded app release");
    expect(() =>
      recordedUpdateAsset({ version: "1.1.0", files: [{ url: "other.zip" }] }, operation)
    ).toThrow("does not contain the recorded app asset");
  });

  // Invariant: Squirrel has staged a verified payload before the shell prepares to quit.
  // Owner: Electron installer adapter. Canonical suite: electron-installer-policy.test.ts.
  it("Should keep the shell open until the native updater is ready", async () => {
    const nativeUpdater = Object.assign(new EventEmitter(), { checkForUpdates: vi.fn() });
    const prepareForRestart = vi.fn(async () => {});
    const quitAndInstall = vi.fn();
    const updater = Object.assign(new EventEmitter(), { quitAndInstall }) as unknown as AppUpdater;
    const installer = new ElectronUpdateInstaller({ updater, nativeUpdater, prepareForRestart });
    const signal = new AbortController().signal;
    const preparing = installer.prepareInstall(signal);
    expect(nativeUpdater.checkForUpdates).toHaveBeenCalledOnce();
    expect(prepareForRestart).not.toHaveBeenCalled();
    expect(quitAndInstall).not.toHaveBeenCalled();
    nativeUpdater.emit("update-downloaded");
    await preparing;
    await installer.quitAndInstall(signal);
    expect(prepareForRestart).toHaveBeenCalledOnce();
    expect(quitAndInstall).toHaveBeenCalledOnce();
  });

  it.each(["error", "abort"] as const)(
    "Should preserve the shell and ignore late native readiness after %s",
    async cause => {
      const nativeUpdater = Object.assign(new EventEmitter(), { checkForUpdates: vi.fn() });
      const prepareForRestart = vi.fn(async () => {});
      const quitAndInstall = vi.fn();
      const updater = Object.assign(new EventEmitter(), {
        quitAndInstall,
      }) as unknown as AppUpdater;
      const installer = new ElectronUpdateInstaller({ updater, nativeUpdater, prepareForRestart });
      const controller = new AbortController();
      const preparing = installer.prepareInstall(controller.signal);
      const rejected = expect(preparing).rejects.toThrow("native staging failed");
      if (cause === "error") nativeUpdater.emit("error", new Error("native staging failed"));
      else controller.abort(new Error("native staging failed"));
      await rejected;
      nativeUpdater.emit("update-downloaded");
      expect(prepareForRestart).not.toHaveBeenCalled();
      expect(quitAndInstall).not.toHaveBeenCalled();
      expect(nativeUpdater.listenerCount("update-downloaded")).toBe(0);
    }
  );

  it("Should cancel the download and refuse completion after its deadline", async () => {
    let token: CancellationToken | undefined;
    let finish!: () => void;
    const blocked = new Promise<void>(resolve => {
      finish = resolve;
    });
    const updater = Object.assign(new EventEmitter(), {
      checkForUpdates: async () => ({
        isUpdateAvailable: true,
        updateInfo: { version: "1.1.0", files: [{ url: operation.asset }] },
      }),
      downloadUpdate: async (cancellation: CancellationToken) => {
        token = cancellation;
        await blocked;
        return [`/tmp/${operation.asset}`];
      },
    }) as unknown as AppUpdater;
    const installer = new ElectronUpdateInstaller({
      updater,
      nativeUpdater: null,
      prepareForRestart: async () => {},
    });
    const controller = new AbortController();
    const downloading = installer.download(operation, async () => {}, controller.signal);
    await vi.waitFor(() => expect(token).toBeDefined());
    controller.abort(new Error("download deadline exceeded"));
    expect(token?.cancelled).toBe(true);
    finish();
    await expect(downloading).rejects.toThrow("download deadline exceeded");
  });

  // Invariant: an unreachable channel feed host cannot block the release the daemon recorded,
  // and the recorded download is verified against the daemon's signed sha256.
  // Owner: Electron installer adapter. Canonical suite: electron-installer-policy.test.ts.
  it("Should download the recorded release when the channel feed is unreachable", async () => {
    const sha256 = "ab".repeat(32);
    const recorded = { ...operation, digest: `sha256:${sha256}` };
    const updater = new FeedUpdater({ channel: new Error("net::ERR_CONNECTION_CLOSED") }, [
      `/tmp/${recorded.asset}`,
    ]);
    const feedFailures: Error[] = [];
    const installer = new ElectronUpdateInstaller({
      updater: updater as unknown as AppUpdater,
      nativeUpdater: null,
      prepareForRestart: async () => {},
      onFeedFallback: error => feedFailures.push(error),
    });

    await expect(
      installer.download(recorded, async () => {}, new AbortController().signal)
    ).resolves.toEqual({ artifactPath: `/tmp/${recorded.asset}`, version: "1.1.0" });
    expect(feedFailures.map(error => error.message)).toEqual(["net::ERR_CONNECTION_CLOSED"]);
    expect(updater.resolved.map(file => ({ url: file.url.href, info: file.info }))).toEqual([
      {
        url: "https://github.com/compozy/compozy/releases/download/v1.1.0/CompozyOS-1.1.0-arm64.zip",
        info: { url: recorded.asset, sha2: sha256 },
      },
    ]);
    // The next attempt reads the channel feed again.
    expect(updater.feed).toBeNull();
  });

  it("Should report both sources when the recorded release download fails too", async () => {
    const updater = new FeedUpdater(
      {
        channel: new Error("net::ERR_CONNECTION_CLOSED"),
        release: new Error("net::ERR_NAME_NOT_RESOLVED"),
      },
      []
    );
    const installer = new ElectronUpdateInstaller({
      updater: updater as unknown as AppUpdater,
      nativeUpdater: null,
      prepareForRestart: async () => {},
    });

    await expect(
      installer.download(
        { ...operation, digest: `sha256:${"cd".repeat(32)}` },
        async () => {},
        new AbortController().signal
      )
    ).rejects.toThrow(
      "The update feed and the recorded app release could not be reached " +
        "(net::ERR_CONNECTION_CLOSED; net::ERR_NAME_NOT_RESOLVED)."
    );
    expect(updater.feed).toBeNull();
  });
});
