import { randomUUID } from "node:crypto";
import { lstat, mkdir, mkdtemp, readFile, rename, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join } from "node:path";

import { create as createTar } from "tar";

import type { DiagnosticReport } from "../state/app-state";
import { publicSafeText } from "../state/public-safe-text";

const MAXIMUM_SOURCE_BYTES = 64 * 1024;
const NON_SHAREABLE = /authorization|credential|password|secret|token|compozy\.db|sqlite/iu;

export interface DiagnosticBundleResult {
  readonly bundle_path: string;
  readonly bytes: number;
}

interface DiagnosticExportOptions {
  readonly home: string;
  readonly logs: string;
  readonly report: DiagnosticReport;
  readonly now?: Date;
}

async function safeLogTail(path: string, bootId: string): Promise<string> {
  let metadata;
  try {
    metadata = await lstat(path);
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return "";
    throw error;
  }
  if (!metadata.isFile() || metadata.isSymbolicLink()) return "";
  const bytes = await readFile(path);
  const tail = bytes.subarray(Math.max(0, bytes.length - MAXIMUM_SOURCE_BYTES)).toString("utf8");
  const output: string[] = [];
  for (const line of tail.split(/\r?\n/u)) {
    let value: unknown;
    try {
      value = JSON.parse(line);
    } catch {
      continue;
    }
    if (!value || typeof value !== "object" || Array.isArray(value)) continue;
    const record = value as Record<string, unknown>;
    if (
      record.boot_id !== bootId ||
      typeof record.timestamp !== "string" ||
      !Number.isFinite(Date.parse(record.timestamp)) ||
      typeof record.level !== "string" ||
      typeof record.message !== "string" ||
      NON_SHAREABLE.test(record.message)
    ) {
      continue;
    }
    output.push(
      JSON.stringify({
        timestamp: new Date(record.timestamp).toISOString(),
        level: record.level.toLowerCase(),
        boot_id: bootId,
        message: publicSafeText(record.message, "Desktop event"),
      })
    );
  }
  return output.length > 0 ? `${output.join("\n")}\n` : "";
}

function bundleName(now: Date): string {
  const stamp = now.toISOString().replaceAll(/[-:.]/gu, "");
  return `compozyos-desktop-diagnostics-${stamp}-${randomUUID()}.tar.gz`;
}

export async function exportDiagnostics(
  options: DiagnosticExportOptions
): Promise<DiagnosticBundleResult> {
  const support = join(options.home, "support-bundles");
  await mkdir(support, { recursive: true, mode: 0o700 });
  const staging = await mkdtemp(join(tmpdir(), "compozy-desktop-diagnostics-"));
  const name = bundleName(options.now ?? new Date());
  const output = join(support, name);
  const temporary = join(support, `.${name}.tmp`);
  try {
    const manifest = {
      schema_version: 1,
      kind: "compozyos_desktop_diagnostics",
      generated_at: (options.now ?? new Date()).toISOString(),
      report: options.report,
    };
    await writeFile(join(staging, "manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`, {
      encoding: "utf8",
      mode: 0o600,
    });
    const entries = ["manifest.json"];
    for (const source of ["desktop.log", "desktop-bootstrap.jsonl"] as const) {
      const tail = await safeLogTail(join(options.logs, source), options.report.boot_id);
      if (!tail) continue;
      await writeFile(join(staging, source), tail, { encoding: "utf8", mode: 0o600 });
      entries.push(source);
    }
    await createTar({ cwd: staging, file: temporary, gzip: true, portable: true }, entries);
    await rename(temporary, output);
    const metadata = await stat(output);
    return { bundle_path: output, bytes: metadata.size };
  } catch (error) {
    await rm(temporary, { force: true });
    throw new Error(`Diagnostics could not be exported to ${basename(output)}.`, { cause: error });
  } finally {
    await rm(staging, { recursive: true, force: true });
  }
}
