import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { extract } from "tar";
import { describe, expect, it } from "vitest";

import { exportDiagnostics } from "../export-diagnostics";

// Invariant: diagnostics remain exportable from shell-owned state when no runtime binary exists.
describe("desktop diagnostic exporter", () => {
  it("Should create a redacted local archive without executing the runtime", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-diagnostics-"));
    const logs = join(home, "logs");
    const extracted = join(home, "extracted");
    await mkdir(logs, { recursive: true });
    await mkdir(extracted);
    const report = {
      schema_version: 1 as const,
      boot_id: "boot-1",
      boot_phase: "error",
      app_version: "1.0.0",
      runtime_version: null,
      runtime_owned: null,
      current_error: { code: "bundle_invalid", safe_message: "Runtime bundle is invalid." },
      previous_crash: null,
    };
    try {
      await writeFile(
        join(logs, "desktop.log"),
        [
          JSON.stringify({
            timestamp: "2026-08-16T12:00:00Z",
            level: "ERROR",
            boot_id: "boot-1",
            message: "Bundle verification failed",
          }),
          JSON.stringify({
            timestamp: "2026-08-16T12:00:01Z",
            level: "ERROR",
            boot_id: "boot-1",
            message: "token=raw-secret",
          }),
        ].join("\n"),
        "utf8"
      );
      const bundle = await exportDiagnostics({
        home,
        logs,
        report,
        now: new Date("2026-08-16T12:00:00Z"),
      });
      expect(bundle.bytes).toBeGreaterThan(0);
      expect(dirname(bundle.bundle_path)).toBe(join(home, "support-bundles"));
      await extract({ file: bundle.bundle_path, cwd: extracted });
      expect(JSON.parse(await readFile(join(extracted, "manifest.json"), "utf8"))).toMatchObject({
        schema_version: 1,
        kind: "compozyos_desktop_diagnostics",
        report: { boot_id: "boot-1" },
      });
      const desktopLog = await readFile(join(extracted, "desktop.log"), "utf8");
      expect(desktopLog).toContain("Bundle verification failed");
      expect(desktopLog).not.toContain("raw-secret");
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });
});
