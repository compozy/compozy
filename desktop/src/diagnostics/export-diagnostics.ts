import { execFile } from "node:child_process";

interface BundleResult {
  readonly bundle_path: string;
  readonly bytes: number;
}

export async function exportDiagnostics(runtimePath: string): Promise<BundleResult> {
  const stdout = await new Promise<string>((resolve, reject) => {
    execFile(
      runtimePath,
      ["daemon", "app-diagnostic-bundle"],
      { encoding: "utf8", timeout: 30_000, maxBuffer: 1024 * 1024 },
      (error, output, stderr) => {
        if (error) {
          reject(
            new Error(stderr.trim() || "Diagnostics could not be exported.", { cause: error })
          );
          return;
        }
        resolve(output);
      }
    );
  });
  let value: unknown;
  try {
    value = JSON.parse(stdout);
  } catch (error) {
    throw new Error("The diagnostic exporter returned invalid JSON.", { cause: error });
  }
  if (!value || typeof value !== "object")
    throw new Error("The diagnostic export result is invalid.");
  const record = value as Record<string, unknown>;
  if (
    typeof record.bundle_path !== "string" ||
    record.bundle_path.trim() === "" ||
    typeof record.bytes !== "number" ||
    !Number.isSafeInteger(record.bytes) ||
    record.bytes < 0
  ) {
    throw new Error("The diagnostic export result is invalid.");
  }
  return { bundle_path: record.bundle_path, bytes: record.bytes };
}
