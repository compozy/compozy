import { chmod, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { delimiter, join } from "node:path";

import { describe, expect, it, vi } from "vitest";

import { linuxDesktopEntry, registerLinuxDesktop } from "../linux-registration";

import { DeepLinkQueue, lastDeepLink, parseDeepLink, productNavigationURL } from "../deep-link";

// Invariant: only absolute, hostless product paths reach the renderer, and a pre-ready burst is last-wins exactly once.
// Owner: desktop deep-link boundary. Canonical suite: deep-link.test.ts.
describe("deep-link policy", () => {
  it("Should accept product paths and preserve their query", () => {
    expect(parseDeepLink("compozyos://open/sessions/abc?tab=logs")).toEqual({
      kind: "product",
      path: "/sessions/abc?tab=logs",
    });
  });

  it.each([
    "compozyos://open/http://evil.com",
    "compozyos://open//evil.com/x",
    "compozyos://open/../etc",
    "compozyos://open/%2e%2e/etc",
    "compozyos://open/..%5c..%5cetc",
    "https://example.com/sessions/abc",
  ])("Should reject hostile target %s to the default view", input => {
    expect(parseDeepLink(input)).toEqual({ kind: "default" });
  });

  it("Should hold the latest link until readiness and consume it once", () => {
    const queue = new DeepLinkQueue();
    queue.push("compozyos://open/sessions/first");
    queue.push("compozyos://open/sessions/last");
    expect(queue.take()).toBeNull();
    expect(queue.setReady()).toBe("/sessions/last");
    expect(queue.take()).toBeNull();
  });

  it("Should carry an explicit default-view intent without changing product paths", () => {
    expect(productNavigationURL("http://127.0.0.1:2123", "/")).toBe(
      "http://127.0.0.1:2123/?_compozy_desktop_default=1"
    );
    expect(productNavigationURL("http://127.0.0.1:2123", "/tasks?state=open")).toBe(
      "http://127.0.0.1:2123/tasks?state=open"
    );
  });

  it("Should extract only the last deep-link argument", () => {
    expect(
      lastDeepLink(["app", "compozyos://open/tasks/first", "--flag", "compozyos://open/tasks/last"])
    ).toBe("compozyos://open/tasks/last");
  });
});

// Invariant: the Linux URI association launches the installed executable with the original URL.
// Owner: desktop protocol registration. Canonical suite: deep-link.test.ts.
describe("Linux URI registration", () => {
  it("Should declare the scheme and launch the current executable directly", () => {
    const entry = linuxDesktopEntry("/home/user/Applications/Compozy OS.AppImage", "1.2.3");
    expect(entry).toContain('Exec="/home/user/Applications/Compozy OS.AppImage" %u\n');
    expect(entry).toContain("MimeType=x-scheme-handler/compozyos;\n");
    expect(entry).toContain("X-AppImage-Version=1.2.3\n");
    expect(entry).not.toContain("app open");
  });
  it("Should escape executable field codes and reject new entry injection", () => {
    expect(linuxDesktopEntry('/tmp/a%u"$.AppImage', "1.2.3")).toContain(
      String.raw`a%%u\\"\\$.AppImage`
    );
    expect(() => linuxDesktopEntry("/tmp/a\nExec=other", "1.2.3")).toThrow("Invalid desktop entry");
  });

  // Invariant: MIME registration terminates a stalled child and releases startup within a bounded time.
  // Owner: desktop protocol registration. Canonical suite: deep-link.test.ts.
  it.skipIf(process.platform === "win32")(
    "Should terminate a stalled MIME registration process",
    async () => {
      const directory = await mkdtemp(join(tmpdir(), "compozy-mime-"));
      const executable = join(directory, "xdg-mime");
      const pidFile = join(directory, "child.pid");
      let childStopped = false;
      let cleanupError: unknown;
      try {
        await writeFile(
          executable,
          `#!${process.execPath}
require("node:fs").writeFileSync(${JSON.stringify(pidFile)}, String(process.pid));
process.on("SIGTERM", () => {});
setInterval(() => {}, 1000);
`
        );
        await chmod(executable, 0o755);
        vi.stubEnv("PATH", directory + delimiter + (process.env.PATH ?? ""));
        await expect(
          registerLinuxDesktop("/tmp/CompozyOS.AppImage", "1.2.3", directory)
        ).rejects.toMatchObject({ killed: true, signal: "SIGKILL" });
        const childPID = Number(await readFile(pidFile, "utf8"));
        expect(() => process.kill(childPID, 0)).toThrowError(
          expect.objectContaining({ code: "ESRCH" })
        );
        childStopped = true;
      } finally {
        vi.unstubAllEnvs();
        // Clean up only the child created by this fixture if an assertion failed before its exit.
        try {
          if (!childStopped) {
            const childPID = Number(await readFile(pidFile, "utf8"));
            process.kill(childPID, "SIGKILL");
          }
        } catch (error) {
          const code = (error as NodeJS.ErrnoException).code;
          if (code !== "ENOENT" && code !== "ESRCH") cleanupError = error;
        }
        await rm(directory, { recursive: true, force: true });
      }
      if (cleanupError) throw cleanupError;
    },
    15_000
  );
});
