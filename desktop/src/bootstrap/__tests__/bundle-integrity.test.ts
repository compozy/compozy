import { createHash } from "node:crypto";
import { chmod, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { executableSha256File, MACH_O_64_LAYOUT } from "../executable-digest";
import { verifyRuntimeBundle } from "../bundle-integrity";

const LC_SEGMENT_64 = 0x19;
const LC_CODE_SIGNATURE = 0x1d;

// Invariant: no bundled runtime content executes unless it matches the build-embedded digest;
// replacing the OS-verified Mach-O signature envelope does not change that content identity.
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

  it("Should preserve Mach-O content identity across code signing", async () => {
    const directory = await mkdtemp(join(tmpdir(), "compozy-desktop-macho-integrity-"));
    const binary = join(directory, "compozy");
    const manifest = join(directory, "runtime-manifest.json");
    const content = Buffer.from("runtime-content", "utf8");
    const machO = (signature: string): Buffer => {
      const segmentCommandBytes = MACH_O_64_LAYOUT.segmentCommandBytes;
      const signatureCommandBytes = MACH_O_64_LAYOUT.signatureCommandBytes;
      const contentOffset =
        MACH_O_64_LAYOUT.headerBytes + segmentCommandBytes + signatureCommandBytes;
      const signatureOffset = contentOffset + content.length;
      const signaturePayload = Buffer.from(signature, "utf8");
      const payload = Buffer.alloc(signatureOffset + signaturePayload.length);
      payload.writeUInt32LE(0xfeedfacf, MACH_O_64_LAYOUT.headerMagic);
      payload.writeUInt32LE(2, MACH_O_64_LAYOUT.headerCommandCount);
      payload.writeUInt32LE(
        segmentCommandBytes + signatureCommandBytes,
        MACH_O_64_LAYOUT.headerCommandBytes
      );
      const segmentCommand = MACH_O_64_LAYOUT.headerBytes;
      payload.writeUInt32LE(LC_SEGMENT_64, segmentCommand + MACH_O_64_LAYOUT.commandKind);
      payload.writeUInt32LE(segmentCommandBytes, segmentCommand + MACH_O_64_LAYOUT.commandSize);
      payload.write("__LINKEDIT", segmentCommand + MACH_O_64_LAYOUT.segmentName, "ascii");
      payload.writeBigUInt64LE(
        BigInt(signaturePayload.length),
        segmentCommand + MACH_O_64_LAYOUT.segmentVMSize
      );
      payload.writeBigUInt64LE(
        BigInt(signatureOffset),
        segmentCommand + MACH_O_64_LAYOUT.segmentFileOffset
      );
      payload.writeBigUInt64LE(
        BigInt(signaturePayload.length),
        segmentCommand + MACH_O_64_LAYOUT.segmentFileSize
      );
      const signatureCommand = MACH_O_64_LAYOUT.headerBytes + segmentCommandBytes;
      payload.writeUInt32LE(LC_CODE_SIGNATURE, signatureCommand + MACH_O_64_LAYOUT.commandKind);
      payload.writeUInt32LE(signatureCommandBytes, signatureCommand + MACH_O_64_LAYOUT.commandSize);
      payload.writeUInt32LE(
        signatureOffset,
        signatureCommand + MACH_O_64_LAYOUT.signatureDataOffset
      );
      payload.writeUInt32LE(
        signaturePayload.length,
        signatureCommand + MACH_O_64_LAYOUT.signatureDataSize
      );
      content.copy(payload, contentOffset);
      signaturePayload.copy(payload, signatureOffset);
      return payload;
    };
    try {
      await writeFile(binary, machO("linker-signature"), { mode: 0o700 });
      const expected = await executableSha256File(binary);
      await writeFile(
        manifest,
        JSON.stringify({
          schema_version: 1,
          asset: "compozy",
          sha256: expected.toString("hex"),
        })
      );

      await writeFile(binary, machO("distribution-signature-with-a-different-size"), {
        mode: 0o700,
      });
      await expect(verifyRuntimeBundle(binary, manifest)).resolves.toBeUndefined();

      const tampered = machO("distribution-signature-with-a-different-size");
      tampered[120] = tampered[120]! ^ 0xff;
      await writeFile(binary, tampered, { mode: 0o700 });
      await expect(verifyRuntimeBundle(binary, manifest)).rejects.toThrow("integrity check");
    } finally {
      await rm(directory, { recursive: true });
    }
  });
});
