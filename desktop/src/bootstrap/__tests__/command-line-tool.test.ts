import {
  access,
  lstat,
  constants,
  mkdir,
  mkdtemp,
  readlink,
  realpath,
  rename,
  rm,
  stat,
  symlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it, vi } from "vitest";

import {
  ensureCommandLineTool,
  linkWithAdministratorPrivileges,
  type CommandLineToolDeps,
} from "../../cli/command-line-tool";
import {
  exposeCommandLineToolOnFirstRun,
  installCommandLineTool,
  type CommandLineToolFlowDeps,
} from "../../cli/command-line-tool-flow";
import { isExecutableFile, runBoundedCommand } from "../../product/login-shell-path";

function dependencies(overrides: Partial<CommandLineToolDeps> = {}): CommandLineToolDeps {
  return {
    runtime: "/home/test/.compozy/bin/compozy",
    home: "/home/test",
    loginShellPath: async () => "/usr/local/bin:/usr/bin:/bin",
    isExecutable: async () => false,
    fs: {
      isFile: async () => true,
      exists: async () => false,
      realpath: async path => path,
      isWritableDirectory: async () => true,
      mkdir: vi.fn(),
      symlink: vi.fn(),
    },
    ...overrides,
  };
}

function flow(): CommandLineToolFlowDeps {
  return {
    ensure: vi.fn().mockResolvedValue({ status: "needs_admin", path: "/usr/local/bin/compozy" }),
    linkWithAdministratorPrivileges: vi.fn(),
    showMessageBox: vi.fn().mockResolvedValue({ response: 1 }),
    attempted: vi.fn().mockResolvedValue(false),
    recordAttempt: vi.fn(),
    log: vi.fn(),
  };
}

describe("desktop command installation", () => {
  it("Should use the installer's writable system directory and the stable runtime", async () => {
    const deps = dependencies();
    expect(await ensureCommandLineTool(deps)).toEqual({
      status: "linked",
      path: "/usr/local/bin/compozy",
    });
    expect(deps.fs.symlink).toHaveBeenCalledWith(deps.runtime, "/usr/local/bin/compozy");
  });

  it("Should preserve the first executable on the user's PATH", async () => {
    const deps = dependencies({
      loginShellPath: async () => "/opt/homebrew/bin:/usr/local/bin",
      isExecutable: async () => true,
    });
    expect(await ensureCommandLineTool(deps)).toEqual({
      status: "existing",
      path: "/opt/homebrew/bin/compozy",
    });
    expect(deps.fs.symlink).not.toHaveBeenCalled();
  });

  it("Should recognize a runtime already on PATH without writes", async () => {
    const deps = dependencies({ isExecutable: async () => true });
    deps.fs.realpath = async () => deps.runtime;
    expect((await ensureCommandLineTool(deps)).status).toBe("current");
    expect(deps.fs.symlink).not.toHaveBeenCalled();
  });

  it("Should refuse to overwrite a conflicting file created during installation", async () => {
    const deps = dependencies();
    deps.fs.symlink = async () => {
      throw Object.assign(new Error("exists"), { code: "EEXIST" });
    };
    expect((await ensureCommandLineTool(deps)).status).toBe("conflict");
  });

  it("Should report a conflicting destination before asking for administrator access", async () => {
    const deps = dependencies();
    deps.fs.isWritableDirectory = async () => false;
    deps.fs.exists = async () => true;
    expect(await ensureCommandLineTool(deps)).toEqual({
      status: "conflict",
      path: "/usr/local/bin/compozy",
    });
  });

  it.each(["EACCES", "ENOTDIR"])(
    "Should offer administrator installation when the user fallback fails with %s",
    async code => {
      const deps = dependencies({
        loginShellPath: async () => "/usr/local/bin:/home/test/.local/bin:/usr/bin:/bin",
      });
      deps.fs.isWritableDirectory = async () => false;
      deps.fs.mkdir = async () => {
        throw Object.assign(new Error("cannot create fallback"), { code });
      };
      expect(await ensureCommandLineTool(deps)).toEqual({
        status: "needs_admin",
        path: "/usr/local/bin/compozy",
      });
      expect(deps.fs.symlink).not.toHaveBeenCalled();
    }
  );

  it("Should report a blocked command without claiming the terminal can run it", async () => {
    const deps = flow();
    vi.mocked(deps.ensure).mockResolvedValue({
      status: "conflict",
      path: "/usr/local/bin/compozy",
    });
    await exposeCommandLineToolOnFirstRun(deps);
    expect(deps.linkWithAdministratorPrivileges).not.toHaveBeenCalled();
    expect(deps.showMessageBox).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "warning",
        message: "An existing file blocks the compozy command",
      })
    );
  });

  it("Should request explicit elevation only for a system directory on PATH", async () => {
    const deps = dependencies();
    deps.fs.isWritableDirectory = async () => false;
    expect((await ensureCommandLineTool(deps)).status).toBe("needs_admin");
    expect(deps.fs.symlink).not.toHaveBeenCalled();
  });

  it.each([null, "/usr/bin:/bin"])(
    "Should report an unavailable installer PATH (%s) without writes",
    async path => {
      const deps = dependencies({ loginShellPath: async () => path });
      expect((await ensureCommandLineTool(deps)).status).toBe("path_required");
      expect(deps.fs.symlink).not.toHaveBeenCalled();
    }
  );

  it("Should leave an external runtime installation alone", async () => {
    const deps = dependencies();
    deps.fs.isFile = async () => false;
    expect((await ensureCommandLineTool(deps)).status).toBe("no_runtime");
    expect(deps.fs.symlink).not.toHaveBeenCalled();
  });

  it("Should quote administrator arguments and never force replacement", async () => {
    const run = vi.fn().mockResolvedValue("");
    await linkWithAdministratorPrivileges(run, "/a user's/runtime", "/usr/local/bin/compozy");
    const args = run.mock.calls[0]![1] as string[];
    expect(args).toContain("/a user's/runtime");
    expect(args.join(" ")).toContain("quoted form of");
    expect(args.join(" ")).not.toContain("ln -sf");
  });

  it("Should honor first-run refusal and not prompt again after a recorded attempt", async () => {
    const deps = flow();
    await exposeCommandLineToolOnFirstRun(deps);
    expect(deps.recordAttempt).toHaveBeenCalledOnce();
    expect(deps.linkWithAdministratorPrivileges).not.toHaveBeenCalled();
    vi.mocked(deps.attempted).mockResolvedValue(true);
    await exposeCommandLineToolOnFirstRun(deps);
    expect(deps.ensure).toHaveBeenCalledOnce();
  });

  it("Should confirm explicit installation and re-read the result after elevation", async () => {
    const deps = flow();
    vi.mocked(deps.ensure)
      .mockResolvedValueOnce({ status: "needs_admin", path: "/usr/local/bin/compozy" })
      .mockResolvedValue({ status: "current", path: "/usr/local/bin/compozy" });
    await installCommandLineTool(deps);
    expect(deps.linkWithAdministratorPrivileges).toHaveBeenCalledOnce();
    expect(deps.ensure).toHaveBeenCalledTimes(2);
    expect(deps.showMessageBox).toHaveBeenCalledWith(
      expect.objectContaining({ message: "The compozy command is ready" })
    );
  });

  it("Should install in the real user fallback, remain idempotent and follow an atomic runtime upgrade", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-desktop-command-"));
    const runtime = join(home, ".compozy/bin/compozy");
    const bin = join(home, ".local/bin");
    try {
      await mkdir(join(home, ".compozy/bin"), { recursive: true });
      await writeFile(runtime, "#!/bin/sh\necho runtime-v1\n", { mode: 0o755 });
      const deps = dependencies({
        home,
        runtime,
        loginShellPath: async () => `${bin}:/usr/bin:/bin`,
        isExecutable: isExecutableFile,
        fs: {
          isFile: async path => (await stat(path)).isFile(),
          exists: async path => {
            try {
              await lstat(path);
              return true;
            } catch {
              return false;
            }
          },
          realpath,
          isWritableDirectory: async path => {
            try {
              await access(path, constants.W_OK);
              return true;
            } catch {
              return false;
            }
          },
          mkdir: async path => {
            await mkdir(path, { recursive: true });
          },
          symlink,
        },
      });
      expect((await ensureCommandLineTool(deps)).status).toBe("linked");
      expect(await readlink(join(bin, "compozy"))).toBe(runtime);
      expect((await ensureCommandLineTool(deps)).status).toBe("current");
      const run = async () =>
        await runBoundedCommand("/bin/sh", ["-c", "compozy --version"], {
          env: { HOME: home, PATH: `${bin}:/usr/bin:/bin` },
          timeoutMs: 2000,
          maxBytes: 1024,
        });
      expect(await run()).toBe("runtime-v1\n");
      await writeFile(`${runtime}.new`, "#!/bin/sh\necho runtime-v2\n", { mode: 0o755 });
      await rename(`${runtime}.new`, runtime);
      expect(await run()).toBe("runtime-v2\n");
      await rm(join(bin, "compozy"));
      await symlink(join(home, "missing-runtime"), join(bin, "compozy"));
      expect((await ensureCommandLineTool(deps)).status).toBe("conflict");
      expect(await readlink(join(bin, "compozy"))).toBe(join(home, "missing-runtime"));
      await rm(join(bin, "compozy"));
      await writeFile(join(bin, "compozy"), "user-owned file", { mode: 0o600 });
      expect((await ensureCommandLineTool(deps)).status).toBe("conflict");
    } finally {
      await rm(home, { recursive: true, force: true });
    }
  });
});
