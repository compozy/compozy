import { execFile } from "node:child_process";
import { chmod, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { createConnection } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { describe, expect, it } from "vitest";

import { ControlServer } from "../control-server";
import { controlEndpoint } from "../control-endpoint";
import { deleteControlToken, rotateControlToken, tokenMatches } from "../control-token";

interface WireResponse {
  readonly schema_version: number;
  readonly id: number;
  readonly result?: unknown;
  readonly error?: { readonly code: string; readonly message: string };
}

async function sendRawRequest(path: string, payload: string): Promise<string> {
  const endpoint = await controlEndpoint(path);
  return await new Promise((resolve, reject) => {
    const socket = createConnection(endpoint);
    let response = "";
    socket.setEncoding("utf8");
    socket.once("error", reject);
    socket.on("data", chunk => (response += chunk));
    socket.once("end", () => {
      resolve(response);
    });
    socket.once("connect", () => socket.end(payload));
  });
}

async function request(path: string, payload: unknown): Promise<WireResponse> {
  return JSON.parse(await sendRawRequest(path, `${JSON.stringify(payload)}\n`)) as WireResponse;
}

async function windowsPermissions(path: string, pipe = false) {
  return JSON.parse(
    (
      await promisify(execFile)(
        join(
          process.env.SystemRoot || "C:\\Windows",
          "System32",
          "WindowsPowerShell",
          "v1.0",
          "powershell.exe"
        ),
        [
          "-NoProfile",
          "-NonInteractive",
          "-Command",
          [
            "$ErrorActionPreference = 'Stop'",
            "$user = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value",
            ...(pipe
              ? [
                  "$pipe = [System.IO.Pipes.NamedPipeClientStream]::new('.', $env:COMPOZY_PRIVATE_DIRECTORY, [System.IO.Pipes.PipeAccessRights]::ReadPermissions, [System.IO.Pipes.PipeOptions]::None, [System.Security.Principal.TokenImpersonationLevel]::Anonymous, [System.IO.HandleInheritability]::None)",
                  "try { $pipe.Connect(5000); $acl = $pipe.GetAccessControl() } finally { $pipe.Dispose() }",
                ]
              : ["$acl = Get-Acl -LiteralPath $env:COMPOZY_PRIVATE_DIRECTORY"]),
            "$rules = $acl.GetAccessRules($true, $true, [System.Security.Principal.SecurityIdentifier])",
            "@{ user = $user; owner = $acl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value; protected = $acl.AreAccessRulesProtected; allowed = @($rules | ForEach-Object { $_.IdentityReference.Value }) } | ConvertTo-Json -Compress",
          ].join("; "),
        ],
        {
          encoding: "utf8",
          env: { ...process.env, COMPOZY_PRIVATE_DIRECTORY: path },
          timeout: 10_000,
          windowsHide: true,
        }
      )
    ).stdout
  ) as { user: string; owner: string; protected: boolean; allowed: string[] };
}

// Invariant: the control channel accepts only the current owner token and keeps files, messages, and methods bounded.
describe("control server", () => {
  it("Should rotate an owner-only token and authenticate a supported method", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-control-"));
    const socketPath = join(home, "app.sock");
    const tokenPath = join(home, "app.token");
    const errors: Error[] = [];
    let server: ControlServer | null = null;
    try {
      const first = await rotateControlToken(tokenPath);
      const second = await rotateControlToken(tokenPath);
      expect(first).not.toBe(second);
      expect((await readFile(tokenPath, "utf8")).trim()).toBe(second);
      server = await ControlServer.start(
        socketPath,
        second,
        async (method, params) => ({ method, params }),
        error => errors.push(error)
      );

      const authorized = await request(socketPath, {
        schema_version: 1,
        id: 7,
        token: second,
        method: "diagnose",
        params: {},
      });
      expect(authorized.result).toEqual({ method: "diagnose", params: {} });

      for (const candidate of [undefined, "wrong", first]) {
        const unauthorized = await request(socketPath, {
          schema_version: 1,
          id: 8,
          token: candidate,
          method: "diagnose",
          params: {},
        });
        expect(unauthorized.error?.code).toBe(
          candidate === undefined ? "app_control_invalid_request" : "app_control_unavailable"
        );
      }
      if (process.platform === "win32") {
        const permissions = await windowsPermissions(home);
        expect(permissions.owner).toBe(permissions.user);
        expect(permissions.protected).toBe(true);
        expect(permissions.allowed.sort()).toEqual(
          [permissions.user, "S-1-5-18", "S-1-5-32-544"].sort()
        );
        const pipe = await windowsPermissions((await controlEndpoint(socketPath)).slice(9), true);
        expect(pipe.protected).toBe(true);
        expect(pipe.allowed).toEqual([pipe.user]);
      } else {
        expect((await stat(tokenPath)).mode & 0o777).toBe(0o600);
        expect((await stat(socketPath)).mode & 0o777).toBe(0o600);
        expect((await stat(home)).mode & 0o777).toBe(0o700);
        if (process.getuid) expect((await stat(socketPath)).uid).toBe(process.getuid());
      }
      expect(errors).toEqual([]);
    } finally {
      if (server) await server.close();
      await deleteControlToken(tokenPath);
      await rm(home, { recursive: true, force: true });
    }
  });

  it("Should keep a valid long-running diagnostic request connected", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-control-long-"));
    const socketPath = join(home, "app.sock");
    const token = await rotateControlToken(join(home, "app.token"));
    const server = await ControlServer.start(
      socketPath,
      token,
      async () => {
        await new Promise(resolve => setTimeout(resolve, 2_100));
        return { bundle_path: "/tmp/diagnostics.tar.gz", bytes: 12 };
      },
      () => undefined
    );
    try {
      const response = await request(socketPath, {
        schema_version: 1,
        id: 12,
        token,
        method: "export_diagnostics",
        params: { consent: true },
      });
      expect(response.result).toEqual({ bundle_path: "/tmp/diagnostics.tar.gz", bytes: 12 });
    } finally {
      await server.close();
      await rm(home, { recursive: true, force: true });
    }
  });

  it("Should refuse oversized and unknown-method requests", async () => {
    const home = await mkdtemp(join(tmpdir(), "compozy-control-bounds-"));
    const socketPath = join(home, "app.sock");
    const token = await rotateControlToken(join(home, "app.token"));
    const server = await ControlServer.start(
      socketPath,
      token,
      async () => ({ ok: true }),
      () => undefined
    );
    try {
      const unknown = await request(socketPath, {
        schema_version: 1,
        id: 9,
        token,
        method: "retired.action",
        params: {},
      });
      expect(unknown.error?.code).toBe("app_control_unknown_method");

      const oversized = await request(socketPath, {
        schema_version: 1,
        id: 10,
        token,
        method: "diagnose",
        params: { value: "x".repeat(65 * 1024) },
      });
      expect(oversized.error?.code).toBe("app_control_invalid_request");

      const duplicate = `${JSON.stringify({ schema_version: 1, id: 11, token, method: "diagnose" })}\n${JSON.stringify({ schema_version: 1, id: 12, token, method: "diagnose" })}\n`;
      expect(
        (JSON.parse(await sendRawRequest(socketPath, duplicate)) as WireResponse).error?.code
      ).toBe("app_control_invalid_request");
    } finally {
      await server.close();
      await rm(home, { recursive: true, force: true });
    }
  });

  // Owner: control startup and private file boundary; canonical suite: control-server.test.ts.
  it.skipIf(process.platform !== "win32")(
    "Should repair recreated or exposed directories and invalid pipe discovery",
    async () => {
      const home = await mkdtemp(join(tmpdir(), "compozy-control-recreated-"));
      const tokenPath = join(home, "app.token");
      const socketPath = join(home, "app.sock");
      let server: ControlServer | null = null;
      try {
        await rotateControlToken(tokenPath);
        await rm(home, { recursive: true });
        await rotateControlToken(tokenPath);
        const permissions = await windowsPermissions(home);
        expect(permissions.protected).toBe(true);
        expect(permissions.allowed.sort()).toEqual(
          [permissions.user, "S-1-5-18", "S-1-5-32-544"].sort()
        );
        // Invariant: token rotation removes inherited access granted after initial protection.
        await promisify(execFile)(
          join(process.env.SystemRoot || "C:\\Windows", "System32", "icacls.exe"),
          [home, "/grant", "*S-1-1-0:(OI)(CI)R"],
          { timeout: 10_000, windowsHide: true }
        );
        expect((await windowsPermissions(home)).allowed).toContain("S-1-1-0");
        const token = await rotateControlToken(tokenPath);
        for (const path of [home, tokenPath]) {
          const repaired = await windowsPermissions(path);
          expect(repaired.allowed.sort()).toEqual(
            [repaired.user, "S-1-5-18", "S-1-5-32-544"].sort()
          );
        }
        await writeFile(socketPath, "corrupted discovery");
        server = await ControlServer.start(
          socketPath,
          token,
          async () => ({ recovered: true }),
          () => undefined
        );
        const response = await request(socketPath, {
          schema_version: 1,
          id: 1,
          token,
          method: "diagnose",
        });
        expect(response.result).toEqual({ recovered: true });
        await expect(
          ControlServer.start(
            socketPath,
            token,
            async () => ({}),
            () => undefined
          )
        ).rejects.toThrow("already listening");
      } finally {
        await server?.close();
        await rm(home, { recursive: true, force: true });
      }
    },
    30_000
  );

  it("Should compare token bytes without accepting length or content differences", () => {
    expect(tokenMatches("abc", "abc")).toBe(true);
    expect(tokenMatches("abc", "abd")).toBe(false);
    expect(tokenMatches("abc", "abcd")).toBe(false);
    expect(tokenMatches("abc", null)).toBe(false);
  });

  it.skipIf(process.platform === "win32")(
    "Should refuse an insecure control directory before binding",
    async () => {
      const home = await mkdtemp(join(tmpdir(), "compozy-control-mode-"));
      await chmod(home, 0o755);
      try {
        await expect(
          ControlServer.start(
            join(home, "app.sock"),
            "token",
            async () => ({}),
            () => undefined
          )
        ).rejects.toThrow("0700");
      } finally {
        await rm(home, { recursive: true, force: true });
      }
    }
  );
});
