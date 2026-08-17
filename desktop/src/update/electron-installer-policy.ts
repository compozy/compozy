import { basename } from "node:path";

import type { AppOperationState } from "./operation-contract";

function normalizedVersion(version: string): string {
  return version.trim().replace(/^v/u, "");
}

export function recordedUpdateAsset(
  updateInfo: { readonly version: string; readonly files: readonly { readonly url: string }[] },
  operation: AppOperationState
): string {
  if (normalizedVersion(updateInfo.version) !== normalizedVersion(operation.to_version)) {
    throw new Error("The update feed does not match the recorded app release.");
  }
  const recordedAsset = operation.asset.trim();
  if (!updateInfo.files.some(file => basename(file.url) === recordedAsset)) {
    throw new Error("The update feed does not contain the recorded app asset.");
  }
  return recordedAsset;
}
