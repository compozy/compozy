import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it } from "vitest";

import {
  isExecutableFile,
  lookPath,
  parseLoginShellOutput,
  resolveLoginShellPath,
  runBoundedCommand,
} from "../login-shell-path";

const posixOnly = process.platform === "win32" ? describe.skip : describe;

describe("login shell PATH output", () => {
  it("Should read the PATH between the last NUL pair and reject empty or multi-line values", () => {
    expect(parseLoginShellOutput("motd\n\0/opt/bin:/usr/bin\0")).toBe("/opt/bin:/usr/bin");
    expect(parseLoginShellOutput("\0first\0noise\0/usr/bin\0")).toBe("/usr/bin");
    expect(parseLoginShellOutput("/usr/bin")).toBeNull();
    expect(parseLoginShellOutput("\0/usr/bin")).toBeNull();
    expect(parseLoginShellOutput("\0 \0")).toBeNull();
    expect(parseLoginShellOutput("\0/a\n/b\0")).toBeNull();
  });
});

posixOnly("login shell PATH resolution", () => {
  it("Should run the inherited login shell without a shell wrapper and return its PATH", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-login-path-"));
    try {
      const path = await resolveLoginShellPath({
        env: { HOME: home, SHELL: "/bin/sh", PATH: "/usr/bin:/bin" },
        accountShell: () => null,
        run: runBoundedCommand,
        isExecutable: isExecutableFile,
        defaultShell: "/bin/false",
      });
      expect(path).toContain("/usr/bin");
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });

  it("Should resolve null when home is unusable or the shell fails", async () => {
    const deps = {
      accountShell: () => "/bin/false",
      run: runBoundedCommand,
      isExecutable: isExecutableFile,
      defaultShell: "/bin/false",
    };
    expect(await resolveLoginShellPath({ ...deps, env: { HOME: "relative" } })).toBeNull();
    expect(
      await resolveLoginShellPath({ ...deps, env: { HOME: tmpdir(), SHELL: "sh" } })
    ).toBeNull();
  });

  it("Should kill a command that outlives its deadline or floods its output", async () => {
    const options = { env: { PATH: "/usr/bin:/bin" }, timeoutMs: 100, maxBytes: 64 };
    await expect(runBoundedCommand("/bin/sh", ["-c", "sleep 5"], options)).rejects.toThrow(
      "timed out"
    );
    await expect(
      runBoundedCommand("/bin/sh", ["-c", "yes"], { ...options, timeoutMs: 2_000 })
    ).rejects.toThrow("exceeded");
    await expect(runBoundedCommand("/nonexistent/editor", [], options)).rejects.toThrow();
  });

  it("Should look up a bare executable on PATH", async () => {
    expect(await lookPath("sh", "relative:/bin", isExecutableFile)).toBe("/bin/sh");
    expect(await lookPath("compozy-no-such-tool", "/bin", isExecutableFile)).toBeNull();
  });
});
