import { writeFile, mkdir, open } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

import {
  Extension,
  isRPCError,
  type ExecuteHookParams,
  type ExtensionOptions,
  type PromptPatch,
  type ShutdownRequest,
} from "@agh/extension-sdk";

const handshakePath = process.env.AGH_PROMPT_ENHANCER_HANDSHAKE_PATH?.trim() ?? "";
const hostCallPath = process.env.AGH_PROMPT_ENHANCER_HOST_CALL_PATH?.trim() ?? "";
const capabilityPath = process.env.AGH_PROMPT_ENHANCER_CAPABILITY_PATH?.trim() ?? "";
const shutdownPath = process.env.AGH_PROMPT_ENHANCER_SHUTDOWN_PATH?.trim() ?? "";

function workspacePrefix(workspace?: string, workspaceID?: string): string {
  return workspace?.trim() || workspaceID?.trim() || "unknown";
}

function applyPromptEnhancement(
  prompt: string,
  workspace?: string,
  workspaceID?: string
): PromptPatch {
  return {
    prompt: `[Workspace: ${workspacePrefix(workspace, workspaceID)}]\n\n${prompt}`,
  };
}

async function ensureParentDir(target: string): Promise<void> {
  if (!target) {
    return;
  }
  await mkdir(path.dirname(target), { recursive: true });
}

async function writeJSON(target: string, value: unknown): Promise<void> {
  if (!target) {
    return;
  }
  await ensureParentDir(target);
  await writeFile(target, `${JSON.stringify(value, null, 2)}\n`, "utf8");
}

async function appendLine(target: string, line: string): Promise<void> {
  if (!target) {
    return;
  }
  await ensureParentDir(target);
  const handle = await open(target, "a");
  try {
    await handle.appendFile(`${line.trim()}\n`, "utf8");
  } finally {
    await handle.close();
  }
}

async function readStdin(): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of process.stdin) {
    chunks.push(typeof chunk === "string" ? Buffer.from(chunk) : chunk);
  }
  return Buffer.concat(chunks).toString("utf8");
}

async function sessionsListWithRetry(fn: () => Promise<unknown>, attempts = 5): Promise<unknown> {
  let lastError: unknown;
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;
      const message = error instanceof Error ? error.message : String(error);
      if (!message.includes("Not initialized")) {
        throw error;
      }
      await new Promise(resolve => {
        setTimeout(resolve, (attempt + 1) * 10);
      });
    }
  }
  throw lastError instanceof Error ? lastError : new Error(String(lastError));
}

export function createExtension(options: ExtensionOptions = {}): Extension {
  const extension = new Extension(
    {
      name: "prompt-enhancer",
      version: "0.1.0",
      capabilities: { provides: ["prompt.provider"] },
      actions: { requires: ["sessions/list"] },
      security: { capabilities: ["session.read"] },
      supported_hook_events: ["prompt.post_assemble"],
    },
    options
  );

  extension.handle(
    "execute_hook",
    async (_ctx, params: ExecuteHookParams<"prompt.post_assemble">) => {
      const prompt = params.payload.prompt ?? "";
      return applyPromptEnhancement(
        prompt,
        params.payload.workspace,
        params.payload.workspace_id
      ) satisfies PromptPatch;
    }
  );

  extension.handle("shutdown", async (_ctx, params: ShutdownRequest) => {
    await appendLine(shutdownPath, `reason=${params.reason ?? "unknown"}`);
    return { acknowledged: true };
  });

  extension.onReady(async (host, session) => {
    await writeJSON(handshakePath, {
      request: session.initializeRequest,
      response: session.initializeResponse,
      pid: process.pid,
    });

    try {
      const sessions = (await sessionsListWithRetry(async () => {
        return await host.sessions.list({});
      })) as unknown[];
      await writeJSON(hostCallPath, {
        session_count: sessions.length,
        sessions,
        pid: process.pid,
      });
    } catch (error) {
      await writeJSON(hostCallPath, {
        error: error instanceof Error ? error.message : String(error),
        pid: process.pid,
      });
    }

    try {
      await host.sessions.create({
        agent: "coder",
        workspace: session.initializeRequest.extension.name,
      });
      await writeJSON(capabilityPath, {
        denied: false,
      });
    } catch (error) {
      await writeJSON(capabilityPath, {
        denied: true,
        code: isRPCError(error) ? error.code : undefined,
        data: isRPCError(error) ? error.data : undefined,
        message: error instanceof Error ? error.message : String(error),
      });
    }
  });

  return extension;
}

async function runHook(name: string): Promise<void> {
  switch (name.trim()) {
    case "prompt_post_assemble": {
      const raw = await readStdin();
      const payload = JSON.parse(raw) as {
        prompt?: string;
        workspace?: string;
        workspace_id?: string;
      };
      const patch = applyPromptEnhancement(
        payload.prompt ?? "",
        payload.workspace,
        payload.workspace_id
      );
      process.stdout.write(`${JSON.stringify(patch)}\n`);
      return;
    }
    default:
      throw new Error(`unsupported hook ${name}`);
  }
}

async function main(): Promise<void> {
  const [mode = "serve", hookName = ""] = process.argv.slice(2);
  if (mode === "hook" || mode === "--hook") {
    await runHook(hookName);
    return;
  }

  await createExtension().start();
}

const entryPoint = process.argv[1];
if (entryPoint && import.meta.url === pathToFileURL(entryPoint).href) {
  void main().catch(error => {
    console.error(error);
    process.exitCode = 1;
  });
}
