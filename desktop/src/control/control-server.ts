import { once } from "node:events";
import { chmod, lstat, realpath, rm, stat } from "node:fs/promises";
import { createConnection, createServer, type Server, type Socket } from "node:net";
import { basename, dirname, join } from "node:path";

import {
  CONTROL_SCHEMA_VERSION,
  parseControlMethod,
  type ControlHandler,
  type ControlRequest,
  type ControlResponse,
  ControlMethodError,
} from "./control-contract";
import { tokenMatches } from "./control-token";

const MAXIMUM_REQUEST_BYTES = 64 * 1024;
const MAXIMUM_CONNECTIONS = 64;
const REQUEST_TIMEOUT_MS = 2_000;
const STALE_SOCKET_PROBE_TIMEOUT_MS = 250;

function errorCode(error: unknown): string | null {
  if (!error || typeof error !== "object" || !("code" in error)) return null;
  return typeof error.code === "string" ? error.code : null;
}

function errorResponse(id: number, code: string, message: string): ControlResponse {
  return { schema_version: CONTROL_SCHEMA_VERSION, id, error: { code, message } };
}

function invalidRequestResponse(): ControlResponse {
  return errorResponse(0, "app_control_invalid_request", "The app control request is invalid.");
}

function parseRequest(data: Buffer): ControlRequest | null {
  let value: unknown;
  try {
    value = JSON.parse(data.toString("utf8"));
  } catch {
    return null;
  }
  if (!value || typeof value !== "object") return null;
  const request = value as Record<string, unknown>;
  if (
    request.schema_version !== CONTROL_SCHEMA_VERSION ||
    typeof request.id !== "number" ||
    !Number.isSafeInteger(request.id) ||
    request.id < 0 ||
    typeof request.method !== "string" ||
    typeof request.token !== "string"
  ) {
    return null;
  }
  return value as ControlRequest;
}

async function writeResponse(socket: Socket, response: ControlResponse): Promise<void> {
  const payload = `${JSON.stringify(response)}\n`;
  if (!socket.write(payload)) await once(socket, "drain");
  socket.end();
}

async function serveConnection(
  socket: Socket,
  token: string,
  handler: ControlHandler
): Promise<void> {
  socket.setTimeout(REQUEST_TIMEOUT_MS, () => socket.destroy());
  let bytes = Buffer.alloc(0);
  for await (const chunk of socket) {
    const next = Buffer.concat([bytes, Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)]);
    if (next.length > MAXIMUM_REQUEST_BYTES) {
      await writeResponse(socket, invalidRequestResponse());
      return;
    }
    const newline = next.indexOf(0x0a);
    if (newline < 0) {
      bytes = next;
      continue;
    }
    if (
      next
        .subarray(newline + 1)
        .toString("utf8")
        .trim() !== ""
    ) {
      await writeResponse(socket, invalidRequestResponse());
      return;
    }
    const request = parseRequest(next.subarray(0, newline));
    if (!request) {
      await writeResponse(socket, invalidRequestResponse());
      return;
    }
    socket.setTimeout(0);
    if (!tokenMatches(token, request.token)) {
      await writeResponse(
        socket,
        errorResponse(
          request.id,
          "app_control_unavailable",
          "The app control request is not authorized."
        )
      );
      return;
    }
    const method = parseControlMethod(request.method.trim());
    if (!method) {
      await writeResponse(
        socket,
        errorResponse(
          request.id,
          "app_control_unknown_method",
          "The app control method is not supported."
        )
      );
      return;
    }
    try {
      const result = await handler(method, request.params ?? {});
      await writeResponse(socket, {
        schema_version: CONTROL_SCHEMA_VERSION,
        id: request.id,
        result,
      });
    } catch (error) {
      const message = error instanceof Error ? error.message : "The app control action failed.";
      const code = error instanceof ControlMethodError ? error.code : "app_control_failed";
      await writeResponse(socket, errorResponse(request.id, code, message));
    }
    return;
  }
}

async function canonicalSocketPath(path: string): Promise<string> {
  const parent = dirname(path);
  const metadata = await lstat(parent);
  if (!metadata.isDirectory() || metadata.isSymbolicLink()) {
    throw new Error("The app control directory is not a real directory.");
  }
  const canonicalParent = await realpath(parent);
  const canonicalMetadata = await stat(canonicalParent);
  if (process.getuid && canonicalMetadata.uid !== process.getuid()) {
    throw new Error("The app control directory is not owned by this user.");
  }
  if ((canonicalMetadata.mode & 0o777) !== 0o700) {
    throw new Error("The app control directory permissions must be 0700.");
  }
  return join(canonicalParent, basename(path));
}

async function socketIsLive(path: string): Promise<boolean> {
  return await new Promise(resolveLive => {
    const probe = createConnection(path);
    const settle = (live: boolean) => {
      probe.destroy();
      resolveLive(live);
    };
    probe.once("connect", () => settle(true));
    probe.once("error", () => settle(false));
    probe.setTimeout(STALE_SOCKET_PROBE_TIMEOUT_MS, () => settle(false));
  });
}

async function removeStaleSocket(path: string): Promise<void> {
  let metadata;
  try {
    metadata = await lstat(path);
  } catch (error) {
    if (errorCode(error) === "ENOENT") return;
    throw error;
  }
  if (!metadata.isSocket()) throw new Error("The app control path is not a socket.");
  if (process.getuid && metadata.uid !== process.getuid()) {
    throw new Error("The stale app control socket is not owned by this user.");
  }
  if (await socketIsLive(path)) throw new Error("Another app control server is already listening.");
  await rm(path);
}

export class ControlServer {
  readonly #server: Server;
  readonly #path: string;

  private constructor(server: Server, path: string) {
    this.#server = server;
    this.#path = path;
  }

  static async start(
    path: string,
    token: string,
    handler: ControlHandler,
    onError: (error: Error) => void
  ): Promise<ControlServer> {
    const canonicalPath = await canonicalSocketPath(path);
    await removeStaleSocket(canonicalPath);
    const server = createServer({ allowHalfOpen: true }, socket => {
      void serveConnection(socket, token, handler).catch(error => {
        onError(error instanceof Error ? error : new Error(String(error)));
        socket.destroy();
      });
    });
    server.maxConnections = MAXIMUM_CONNECTIONS;
    await new Promise<void>((resolveListen, reject) => {
      server.once("error", reject);
      server.listen(canonicalPath, () => {
        server.off("error", reject);
        resolveListen();
      });
    });
    await chmod(canonicalPath, 0o600);
    return new ControlServer(server, canonicalPath);
  }

  async close(): Promise<void> {
    await new Promise<void>((resolveClose, reject) => {
      this.#server.close(error => (error ? reject(error) : resolveClose()));
    });
    await rm(this.#path, { force: true });
  }
}
