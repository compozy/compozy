import { describe, expect, it } from "vitest";

import { recordedUpdateAsset } from "../electron-installer-policy";
import type { AppOperationState } from "../operation-contract";

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

// Invariant: installer feed identity must exactly match the durable app operation.
describe("electron installer feed policy", () => {
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
});
