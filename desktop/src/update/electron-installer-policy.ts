import { basename } from "node:path";

import type { AppOperationState } from "./operation-contract";

function normalizedVersion(version: string): string {
  return version.trim().replace(/^v/u, "");
}

/** Reports whether a feed version is the release the daemon recorded for this operation. */
export function isRecordedVersion(version: string, operation: AppOperationState): boolean {
  return normalizedVersion(version) === normalizedVersion(operation.to_version);
}

export function recordedUpdateAsset(
  updateInfo: { readonly version: string; readonly files: readonly { readonly url: string }[] },
  operation: AppOperationState
): string {
  if (!isRecordedVersion(updateInfo.version, operation)) {
    throw new Error("The update feed does not match the recorded app release.");
  }
  const recordedAsset = operation.asset.trim();
  if (!updateInfo.files.some(file => basename(file.url) === recordedAsset)) {
    throw new Error("The update feed does not contain the recorded app asset.");
  }
  return recordedAsset;
}
