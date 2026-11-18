import { basename } from "node:path";

import {
  Provider,
  type ResolvedUpdateFileInfo,
  type UpdateFileInfo,
  type UpdateInfo,
} from "electron-updater";
import type { ProviderRuntimeOptions } from "electron-updater/out/providers/Provider";

import type { AppOperationState } from "./operation-contract";

/** The daemon plans app updates from this repository's GitHub releases only. */
const RELEASE_DOWNLOAD_BASE = "https://github.com/compozy/compozy/releases/download/";

/** A custom electron-updater feed that never fetches a channel manifest. */
export type RecordedReleaseFeed = {
  readonly provider: "custom";
  readonly updateProvider: new (
    options: unknown,
    updater: unknown,
    runtimeOptions: ProviderRuntimeOptions
  ) => Provider<UpdateInfo>;
};

/**
 * Builds the update feed from the daemon's recorded release instead of the channel manifest.
 *
 * The asset lives on the same GitHub release the daemon already downloaded `checksums.txt`
 * from, and the digest is the sha256 the daemon took from that signed catalog, so
 * electron-updater verifies the download against it.
 */
export function recordedReleaseFeed(operation: AppOperationState): RecordedReleaseFeed {
  const asset = operation.asset.trim();
  const tag = operation.release_tag.trim();
  const sha256 = /^sha256:([0-9a-f]{64})$/u.exec(operation.digest.trim().toLowerCase())?.[1];
  if (!asset || basename(asset) !== asset || !tag || tag.includes("/") || !sha256) {
    throw new Error("The recorded app release cannot be downloaded directly.");
  }
  const version = operation.to_version.trim().replace(/^v/u, "");
  const releaseURL = new URL(`${encodeURIComponent(tag)}/`, RELEASE_DOWNLOAD_BASE);

  class RecordedReleaseProvider extends Provider<UpdateInfo> {
    constructor(_options: unknown, _updater: unknown, runtimeOptions: ProviderRuntimeOptions) {
      super(runtimeOptions);
    }

    override async getLatestVersion(): Promise<UpdateInfo> {
      // electron-updater verifies `sha2` as hex sha256 when a file has no sha512.
      const file = { url: asset, sha2: sha256 } as unknown as UpdateFileInfo;
      return { version, files: [file], path: asset, releaseDate: "" } as unknown as UpdateInfo;
    }

    override resolveFiles(updateInfo: UpdateInfo): ResolvedUpdateFileInfo[] {
      return updateInfo.files.map(info => ({
        url: new URL(encodeURIComponent(info.url), releaseURL),
        info,
      }));
    }
  }

  return { provider: "custom", updateProvider: RecordedReleaseProvider };
}
