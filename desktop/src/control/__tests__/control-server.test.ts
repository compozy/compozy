import { getuid } from "node:process";
import { chmod, mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { createConnection } from "node:net";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { ControlServer } from "../control-server";
import { deleteControlToken, rotateControlToken, tokenMatches } from "../control-token";

interface WireResponse {
  readonly schema_version: number;
  readonly id: number;
  readonly result?: unknown;
  readonly error?: { readonly code: string; readonly message: string };
}

async function request(path: string, payload: unknown): Promise<WireResponse> {
  return await new Promise((resolve, reject) => {
    const socket = createConnection(path);
    let response = "";
    socket.setEncoding("utf8");
    socket.once("error", reject);
    socket.on("data", chunk => (response += chunk));
    socket.once("end", () => {
      try {
        resolve(JSON.parse(response) as WireResponse);
      } catch (error) {
        reject(error);
      }
    });
    socket.once("connect", () => socket.end(`${JSON.stringify(payload)}\n`));
  });
}

async function rawRequest(path: string, payload: string): Promise<WireResponse> {
  return await new Promise((resolve, reject) => {
    const socket = createConnection(path);
    let response = "";
    socket.setEncoding("utf8");
    socket.once("error", reject);
    socket.on("data", chunk => (response += chunk));
    socket.once("end", () => resolve(JSON.parse(response) as WireResponse));
    socket.once("connect", () => socket.end(payload));
  });
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
          candidate === undefined ? "app_control_invalid_request" : "app_control_unauthorized"
        );
      }
      expect((await stat(tokenPath)).mode & 0o777).toBe(0o600);
      expect((await stat(socketPath)).mode & 0o777).toBe(0o600);
      expect((await stat(home)).mode & 0o777).toBe(0o700);
      if (getuid) expect((await stat(socketPath)).uid).toBe(getuid());
      expect(errors).toEqual([]);
    } finally {
      if (server) await server.close();
      await deleteControlToken(tokenPath);
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
      expect((await rawRequest(socketPath, duplicate)).error?.code).toBe(
        "app_control_invalid_request"
      );
    } finally {
      await server.close();
      await rm(home, { recursive: true, force: true });
    }
  });

  it("Should compare token bytes without accepting length or content differences", () => {
    expect(tokenMatches("abc", "abc")).toBe(true);
    expect(tokenMatches("abc", "abd")).toBe(false);
    expect(tokenMatches("abc", "abcd")).toBe(false);
    expect(tokenMatches("abc", null)).toBe(false);
  });

  it("Should refuse an insecure control directory before binding", async () => {
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
  });
});
