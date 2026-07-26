import { readFile } from "node:fs/promises";
import process from "node:process";

import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { StdioClientTransport } from "@modelcontextprotocol/sdk/client/stdio.js";

/** The single stdio MCP server the daemon injects into every ACP session. */
export const HOSTED_MCP_SERVER_NAME = "agh-hosted-tools";

export interface HostedMcpStdioDescriptor {
  command: string;
  args: string[];
  env: Record<string, string>;
}

interface DiagnosticsEnvVar {
  name?: string;
  value?: string;
}

// The ACP McpServer union serializes its stdio variant flat: `name`/`command`/`args`/`env`
// live directly on the server object (no `stdio` wrapper).
interface DiagnosticsMcpServer {
  name?: string;
  command?: string;
  args?: string[];
  env?: DiagnosticsEnvVar[];
}

interface DiagnosticsRecord {
  lifecycle_event?: string;
  mcp_servers?: DiagnosticsMcpServer[];
}

function extractHostedStdio(record: DiagnosticsRecord): HostedMcpStdioDescriptor | null {
  for (const server of record.mcp_servers ?? []) {
    if (server.name !== HOSTED_MCP_SERVER_NAME || typeof server.command !== "string") {
      continue;
    }
    const env: Record<string, string> = {};
    for (const entry of server.env ?? []) {
      if (entry?.name) {
        env[entry.name] = entry.value ?? "";
      }
    }
    return { command: server.command, args: [...(server.args ?? [])], env };
  }
  return null;
}

/**
 * Poll the acpmock diagnostics jsonl for the `session_new` record carrying the hosted MCP
 * stdio descriptor (`command`/`args`/`env`). The diagnostics file is per mock agent and this
 * run creates a single session with it, so the first hosted `session_new` record is ours.
 * The `--bind-nonce` in the args is single-use and TTL-bound, so callers should connect
 * promptly. The raw diagnostics are never surfaced in errors because they carry the nonce.
 */
export async function readHostedMcpDescriptor(
  diagnosticsPath: string,
  options: { timeoutMs?: number; pollMs?: number } = {}
): Promise<HostedMcpStdioDescriptor> {
  const timeoutMs = options.timeoutMs ?? 15_000;
  const deadline = Date.now() + timeoutMs;
  const pollMs = options.pollMs ?? 200;
  let recordCount = 0;
  const seenEvents = new Set<string>();
  while (Date.now() < deadline) {
    let raw = "";
    try {
      raw = await readFile(diagnosticsPath, "utf8");
    } catch {
      raw = "";
    }
    recordCount = 0;
    seenEvents.clear();
    for (const line of raw.split("\n")) {
      const trimmed = line.trim();
      if (trimmed === "") {
        continue;
      }
      let record: DiagnosticsRecord;
      try {
        record = JSON.parse(trimmed) as DiagnosticsRecord;
      } catch {
        continue;
      }
      recordCount += 1;
      if (record.lifecycle_event) {
        seenEvents.add(record.lifecycle_event);
      }
      if (record.lifecycle_event !== "session_new") {
        continue;
      }
      const descriptor = extractHostedStdio(record);
      if (descriptor) {
        return descriptor;
      }
    }
    await delay(pollMs);
  }
  throw new Error(
    `hosted MCP stdio server (${HOSTED_MCP_SERVER_NAME}) not found in ${diagnosticsPath} after ` +
      `${timeoutMs}ms; ${recordCount} record(s), lifecycle events: [${[...seenEvents].join(", ")}]`
  );
}

export interface HostedMcpConnection {
  client: Client;
  /** Close the client and force-kill the stdio child so no process can leak. */
  close(): Promise<void>;
}

/** Spawn a Node MCP client bound to the daemon's hosted MCP stdio server for one session. */
export async function connectHostedMcpClient(
  descriptor: HostedMcpStdioDescriptor
): Promise<HostedMcpConnection> {
  const transport = new StdioClientTransport({
    command: descriptor.command,
    args: descriptor.args,
    env: { ...(process.env.PATH ? { PATH: process.env.PATH } : {}), ...descriptor.env },
  });
  const client = new Client({ name: "agh-tool-approval-e2e", version: "1.0.0" });
  const close = async (): Promise<void> => {
    const pid = transport.pid;
    const errors: unknown[] = [];
    try {
      await client.close();
    } catch (error) {
      errors.push(error);
    }
    // The stdio child does not reliably exit on transport close; force it and prove it is gone
    // rather than masking a client.close error.
    try {
      await ensureChildExited(pid);
    } catch (error) {
      errors.push(error);
    }
    if (errors.length === 1) {
      throw errors[0];
    }
    if (errors.length > 1) {
      throw new AggregateError(errors, "hosted MCP client teardown errored");
    }
  };
  try {
    await client.connect(transport);
  } catch (connectError) {
    // A failed connect may have already spawned the stdio child; close it so it cannot leak,
    // and surface any cleanup failure alongside the connect error rather than swallowing it.
    try {
      await close();
    } catch (closeError) {
      throw new AggregateError(
        [connectError, closeError],
        "hosted MCP client connect failed and its cleanup errored"
      );
    }
    throw connectError;
  }
  return { client, close };
}

/**
 * Aggregate the two hosted-MCP teardown steps — close the connection, then drain the pending call —
 * so neither is skipped and a leaked stdio child (a close failure) is never masked by the
 * pending-call drain. The expected early-close rejection of an in-flight call is suppressed only when
 * a close was attempted; a drain failure without a close attempt surfaces. Kept outside a `finally`
 * block so surfacing a teardown failure does not trip `no-unsafe-finally`.
 */
export async function teardownHostedMcp(
  connection: HostedMcpConnection | null,
  toolCall: ReturnType<Client["callTool"]> | undefined
): Promise<void> {
  const cleanupErrors: unknown[] = [];
  let closeAttempted = false;
  if (connection) {
    closeAttempted = true;
    try {
      await connection.close();
    } catch (error) {
      cleanupErrors.push(error);
    }
  }
  if (toolCall) {
    try {
      await toolCall;
    } catch (error) {
      if (!closeAttempted) {
        cleanupErrors.push(error);
      }
    }
  }
  if (cleanupErrors.length === 1) {
    throw cleanupErrors[0];
  }
  if (cleanupErrors.length > 1) {
    throw new AggregateError(cleanupErrors, "hosted MCP client teardown errored");
  }
}

/**
 * Force-kill the stdio child and poll until it is actually gone. `client.close()` escalates
 * to SIGKILL but returns without confirming exit, so ownership requires proving the PID is
 * reaped (ESRCH). A child still alive after the timeout is a teardown failure.
 */
async function ensureChildExited(pid: number | null): Promise<void> {
  if (pid === null) {
    return;
  }
  try {
    process.kill(pid, "SIGKILL");
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ESRCH") {
      return;
    }
    throw error;
  }
  const deadline = Date.now() + 5_000;
  while (Date.now() < deadline) {
    try {
      process.kill(pid, 0);
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code === "ESRCH") {
        return;
      }
      throw error;
    }
    await delay(50);
  }
  throw new Error(`hosted MCP stdio child ${pid} did not exit after SIGKILL`);
}

function delay(ms: number): Promise<void> {
  return new Promise(resolve => setTimeout(resolve, ms));
}
