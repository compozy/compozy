import { createHash } from "node:crypto";
import { chmod, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { verifyRuntimeBundle } from "../bundle-integrity";

// Invariant: no bundled runtime byte executes unless it matches the build-embedded digest manifest.
describe("runtime bundle integrity", () => {
  it("Should accept only the exact bundled payload", async () => {
    const directory = await mkdtemp(join(tmpdir(), "compozy-desktop-integrity-"));
    const binary = join(directory, ".integrity-fixture-bin");
    const manifest = join(directory, ".integrity-fixture-manifest.json");
    try {
      const payload = "verified-runtime";
      await writeFile(binary, payload, { mode: 0o700 });
      await chmod(binary, 0o700);
      await writeFile(
        manifest,
        JSON.stringify({
          schema_version: 1,
          asset: ".integrity-fixture-bin",
          sha256: createHash("sha256").update(payload).digest("hex"),
        })
      );
      await expect(verifyRuntimeBundle(binary, manifest)).resolves.toBeUndefined();
      await writeFile(binary, "tampered-runtime");
      await expect(verifyRuntimeBundle(binary, manifest)).rejects.toThrow("integrity check");
    } finally {
      await rm(directory, { recursive: true });
    }
  });
});
