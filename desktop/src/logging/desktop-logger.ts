import { appendFile, chmod, mkdir } from "node:fs/promises";
import { dirname } from "node:path";

export class DesktopLogger {
  readonly #path: string;
  #writes = Promise.resolve();

  constructor(path: string) {
    this.#path = path;
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
    const line = `${JSON.stringify({ time: new Date().toISOString(), level, message, ...fields })}\n`;
    this.#writes = this.#writes
      .then(async () => {
        const parent = dirname(this.#path);
        await mkdir(parent, { recursive: true, mode: 0o700 });
        await chmod(parent, 0o700);
        await appendFile(this.#path, line, { encoding: "utf8", mode: 0o600 });
      })
      .catch(error => {
        process.stderr.write(
          `desktop log write failed: ${error instanceof Error ? error.message : String(error)}\n`
        );
      });
  }
}
