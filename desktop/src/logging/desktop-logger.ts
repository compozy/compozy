import { appendFile, rename, rm, stat } from "node:fs/promises";
import { dirname } from "node:path";

import { publicSafeText } from "../state/public-safe-text";
import { ensurePrivateDirectory } from "../files/private-directory";

const MAXIMUM_LOG_BYTES = 1024 * 1024;
const REDACTED = "[redacted]";

function sanitizeField(key: string, value: unknown, depth = 0): unknown {
  if (/authorization|credential|password|secret|token/iu.test(key)) return REDACTED;
  if (typeof value === "string") return publicSafeText(value, "");
  if (value === null || typeof value === "number" || typeof value === "boolean") return value;
  if (depth >= 2) return String(value);
  if (Array.isArray(value))
    return value.slice(0, 32).map(item => sanitizeField("item", item, depth + 1));
  if (typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value)
        .slice(0, 32)
        .map(([childKey, childValue]) => [childKey, sanitizeField(childKey, childValue, depth + 1)])
    );
  }
  return String(value);
}

export class DesktopLogger {
  readonly #path: string;
  readonly #bootId: string;
  #writes = Promise.resolve();

  constructor(path: string, bootId: string) {
    this.#path = path;
    this.#bootId = bootId;
  }

  info(message: string, fields: Record<string, unknown> = {}): void {
    this.#write("INFO", message, fields);
  }

  error(message: string, error: unknown, fields: Record<string, unknown> = {}): void {
    this.#write("ERROR", message, {
      ...fields,
      error: error instanceof Error ? error.message : String(error),
    });
  }

  async flush(): Promise<void> {
    await this.#writes;
  }

  #write(level: "INFO" | "ERROR", message: string, fields: Record<string, unknown>): void {
    const safeFields = Object.fromEntries(
      Object.entries(fields).map(([key, value]) => [key, sanitizeField(key, value)])
    );
    const line = `${JSON.stringify({
      timestamp: new Date().toISOString(),
      level,
      boot_id: this.#bootId,
      message: publicSafeText(message, "Desktop event"),
      ...safeFields,
    })}\n`;
    this.#writes = this.#writes
      .then(async () => {
        const parent = dirname(this.#path);
        await ensurePrivateDirectory(parent);
        let size = 0;
        try {
          size = (await stat(this.#path)).size;
        } catch (error) {
          if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) throw error;
        }
        if (size + Buffer.byteLength(line) > MAXIMUM_LOG_BYTES) {
          const rotatedPath = `${this.#path}.1`;
          await rm(rotatedPath, { force: true });
          await rename(this.#path, rotatedPath);
        }
        await appendFile(this.#path, line, { encoding: "utf8", mode: 0o600 });
      })
      .catch(error => {
        process.stderr.write(
          `desktop log write failed: ${error instanceof Error ? error.message : String(error)}\n`
        );
      });
  }
}
