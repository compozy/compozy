import { readFile } from "node:fs/promises";
import { homedir } from "node:os";
import { join, resolve } from "node:path";

import { CompozyConfigError } from "./errors.js";

export interface DaemonInfo {
  pid: number;
  port: number;
  http_host?: string;
}

export function resolveHome(home?: string): string {
  return resolve(home ?? (process.env.COMPOZY_HOME?.trim() || join(homedir(), ".compozy")));
}

export async function readDaemonInfo(home: string): Promise<DaemonInfo | undefined> {
  const path = join(home, "daemon.json");
  let content: string;
  try {
    content = await readFile(path, "utf8");
  } catch (cause) {
    if ((cause as NodeJS.ErrnoException).code === "ENOENT") return undefined;
    throw new CompozyConfigError(`Cannot read daemon record ${path}.`, { cause });
  }
  let value: unknown;
  try {
    value = JSON.parse(content);
  } catch (cause) {
    throw new CompozyConfigError(`Invalid daemon record ${path}: expected JSON.`, { cause });
  }
  if (!value || typeof value !== "object") {
    throw new CompozyConfigError(`Invalid daemon record ${path}.`);
  }
  const record = value as Record<string, unknown>;
  if (
    typeof record.pid !== "number" ||
    !Number.isSafeInteger(record.pid) ||
    record.pid <= 0 ||
    typeof record.port !== "number" ||
    !Number.isInteger(record.port) ||
    record.port < 1 ||
    record.port > 65535 ||
    (record.http_host !== undefined && typeof record.http_host !== "string")
  ) {
    throw new CompozyConfigError(
      `Invalid daemon record ${path}: expected a positive pid and HTTP port.`
    );
  }
  const info: DaemonInfo = { pid: record.pid, port: record.port };
  if (typeof record.http_host === "string") info.http_host = record.http_host;
  // Validate the host before allowing it to become a request target.
  daemonBaseUrl(info);
  return info;
}

export function daemonBaseUrl(info: DaemonInfo): string {
  const host = info.http_host?.trim() || "127.0.0.1";
  const dialHost =
    host === "0.0.0.0" ? "127.0.0.1" : host === "::" || host === "[::]" ? "::1" : host;
  const authority =
    dialHost.includes(":") && !dialHost.startsWith("[") ? `[${dialHost}]` : dialHost;
  const candidate = `http://${authority}:${info.port}`;
  const parsed = validUrl(candidate, "daemon.json http_host");
  if (new URL(parsed).pathname !== "/") {
    throw new CompozyConfigError(
      "Invalid daemon.json http_host: expected a hostname or IP address."
    );
  }
  return parsed;
}

export async function discover(options: { baseUrl?: string; home?: string } = {}): Promise<{
  baseUrl: string;
  token?: string;
}> {
  let baseUrl: string;
  if (options.baseUrl !== undefined) {
    baseUrl = validUrl(options.baseUrl, "baseUrl");
  } else if (process.env.COMPOZY_URL) {
    baseUrl = validUrl(process.env.COMPOZY_URL, "COMPOZY_URL");
  } else {
    const info = await readDaemonInfo(resolveHome(options.home));
    baseUrl = info ? daemonBaseUrl(info) : "http://127.0.0.1:2123";
  }
  const token = process.env.COMPOZY_APP_TOKEN;
  return token ? { baseUrl, token } : { baseUrl };
}

function validUrl(value: string, source: string): string {
  try {
    const url = new URL(value);
    if (
      (url.protocol !== "http:" && url.protocol !== "https:") ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    ) {
      throw new Error("expected an HTTP(S) URL without credentials, query, or fragment");
    }
    return url.toString().replace(/\/$/, "");
  } catch (cause) {
    throw new CompozyConfigError(`Invalid ${source}: expected an HTTP(S) URL.`, { cause });
  }
}
