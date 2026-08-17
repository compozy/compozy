import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { open, type FileHandle } from "node:fs/promises";

const MACH_O_64_LITTLE_ENDIAN = 0xfeedfacf;
const MAXIMUM_LOAD_COMMAND_BYTES = 1024 * 1024;
const LC_SEGMENT_64 = 0x19;
const LC_CODE_SIGNATURE = 0x1d;

export const MACH_O_64_LAYOUT = {
  headerBytes: 32,
  headerMagic: 0,
  headerCommandCount: 16,
  headerCommandBytes: 20,
  commandKind: 0,
  commandSize: 4,
  segmentName: 8,
  segmentNameBytes: 16,
  segmentVMSize: 32,
  segmentFileOffset: 40,
  segmentFileSize: 48,
  segmentCommandBytes: 72,
  signatureDataOffset: 8,
  signatureDataSize: 12,
  signatureCommandBytes: 16,
} as const;

interface MachODigestPlan {
  readonly contentEnd: number;
  readonly header: Buffer;
}

async function readExact(
  handle: FileHandle,
  buffer: Buffer,
  offset: number,
  length: number,
  position: number
): Promise<void> {
  let read = 0;
  while (read < length) {
    const result = await handle.read(buffer, offset + read, length - read, position + read);
    if (result.bytesRead === 0) throw new Error("The Mach-O load commands are truncated.");
    read += result.bytesRead;
  }
}

function segmentName(header: Buffer, commandOffset: number): string {
  const name = header.toString(
    "ascii",
    commandOffset + MACH_O_64_LAYOUT.segmentName,
    commandOffset + MACH_O_64_LAYOUT.segmentName + MACH_O_64_LAYOUT.segmentNameBytes
  );
  const terminator = name.indexOf(String.fromCharCode(0));
  return terminator === -1 ? name : name.slice(0, terminator);
}

async function machODigestPlan(path: string): Promise<MachODigestPlan | null> {
  const handle = await open(path, "r");
  try {
    const file = await handle.stat();
    if (file.size < MACH_O_64_LAYOUT.headerBytes) return null;
    const prefix = Buffer.alloc(MACH_O_64_LAYOUT.headerBytes);
    await readExact(handle, prefix, 0, prefix.length, 0);
    if (prefix.readUInt32LE(0) !== MACH_O_64_LITTLE_ENDIAN) return null;

    const commandCount = prefix.readUInt32LE(MACH_O_64_LAYOUT.headerCommandCount);
    const commandBytes = prefix.readUInt32LE(MACH_O_64_LAYOUT.headerCommandBytes);
    if (commandBytes > MAXIMUM_LOAD_COMMAND_BYTES) {
      throw new Error("The Mach-O load commands exceed the integrity limit.");
    }
    const header = Buffer.alloc(MACH_O_64_LAYOUT.headerBytes + commandBytes);
    prefix.copy(header);
    await readExact(
      handle,
      header,
      MACH_O_64_LAYOUT.headerBytes,
      commandBytes,
      MACH_O_64_LAYOUT.headerBytes
    );

    const linkEditCommands: number[] = [];
    let signatureCommand: number | null = null;
    let commandOffset = MACH_O_64_LAYOUT.headerBytes;
    for (let index = 0; index < commandCount; index += 1) {
      if (commandOffset + 8 > header.length) {
        throw new Error("The Mach-O load command table is invalid.");
      }
      const command = header.readUInt32LE(commandOffset + MACH_O_64_LAYOUT.commandKind);
      const commandSize = header.readUInt32LE(commandOffset + MACH_O_64_LAYOUT.commandSize);
      if (commandSize < 8 || commandOffset + commandSize > header.length) {
        throw new Error("The Mach-O load command size is invalid.");
      }
      if (command === LC_SEGMENT_64 && segmentName(header, commandOffset) === "__LINKEDIT") {
        if (commandSize < MACH_O_64_LAYOUT.segmentCommandBytes) {
          throw new Error("The Mach-O __LINKEDIT command is invalid.");
        }
        linkEditCommands.push(commandOffset);
      }
      if (command === LC_CODE_SIGNATURE) {
        if (commandSize < MACH_O_64_LAYOUT.signatureCommandBytes || signatureCommand !== null) {
          throw new Error("The Mach-O code-signature command is invalid.");
        }
        signatureCommand = commandOffset;
      }
      commandOffset += commandSize;
    }
    if (commandOffset !== header.length) {
      throw new Error("The Mach-O load command table has trailing bytes.");
    }
    if (signatureCommand === null) return null;

    const contentEnd = header.readUInt32LE(signatureCommand + MACH_O_64_LAYOUT.signatureDataOffset);
    const signatureBytes = header.readUInt32LE(
      signatureCommand + MACH_O_64_LAYOUT.signatureDataSize
    );
    if (contentEnd < header.length || contentEnd + signatureBytes !== file.size) {
      throw new Error("The Mach-O code-signature envelope is invalid.");
    }

    // Signing replaces the signature envelope and rewrites only these size
    // fields. The OS verifies the ignored envelope before this binary runs.
    header.writeUInt32LE(0, signatureCommand + MACH_O_64_LAYOUT.signatureDataSize);
    for (const offset of linkEditCommands) {
      header.writeBigUInt64LE(0n, offset + MACH_O_64_LAYOUT.segmentVMSize);
      header.writeBigUInt64LE(0n, offset + MACH_O_64_LAYOUT.segmentFileSize);
    }
    return { contentEnd, header };
  } finally {
    await handle.close();
  }
}

async function updateFromFile(
  digest: ReturnType<typeof createHash>,
  path: string,
  start?: number,
  end?: number
): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    const input = createReadStream(path, {
      ...(start === undefined ? {} : { start }),
      ...(end === undefined ? {} : { end }),
    });
    input.on("data", chunk => digest.update(chunk));
    input.once("error", reject);
    input.once("end", resolve);
  });
}

/** Hashes executable content while excluding the OS-verified Mach-O signature envelope. */
export async function executableSha256File(path: string): Promise<Buffer> {
  const digest = createHash("sha256");
  const plan = await machODigestPlan(path);
  if (!plan) {
    await updateFromFile(digest, path);
    return digest.digest();
  }
  digest.update(plan.header);
  if (plan.header.length < plan.contentEnd) {
    await updateFromFile(digest, path, plan.header.length, plan.contentEnd - 1);
  }
  return digest.digest();
}
