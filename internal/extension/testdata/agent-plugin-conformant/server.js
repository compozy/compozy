import readline from "node:readline";
import fs from "node:fs";
import path from "node:path";

const lines = readline.createInterface({ input: process.stdin });

const respond = (id, result) => {
  process.stdout.write(`${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`);
};

const respondError = (id, code, message) => {
  process.stdout.write(`${JSON.stringify({ jsonrpc: "2.0", id, error: { code, message } })}\n`);
};

lines.on("line", line => {
  let request;
  try {
    request = JSON.parse(line);
  } catch {
    return;
  }
  if (request.id === undefined) {
    return;
  }
  if (request.method === "initialize") {
    respond(request.id, {
      protocolVersion: "2026-07-28",
      capabilities: { tools: {} },
      serverInfo: { name: "portable-runtime-fixture", version: "1.0.0" },
    });
    return;
  }
  if (request.method === "tools/list") {
    respond(request.id, {
      tools: [
        {
          name: "echo_environment",
          description: "Echoes the portable runtime environment.",
          inputSchema: { type: "object", additionalProperties: false },
        },
      ],
    });
    return;
  }
  if (request.method === "tools/call" && request.params?.name === "echo_environment") {
    try {
      const statePath = process.argv[2];
      if (!statePath) throw new Error("state path is required");
      fs.mkdirSync(path.dirname(statePath), { recursive: true });
      const payload = {
        pluginRoot: process.env.PLUGIN_ROOT,
        pluginData: process.env.PLUGIN_DATA,
        mode: process.env.MODE,
        statePath,
        unknownArg: process.argv[3],
        cwd: process.cwd(),
        writable: false,
      };
      fs.writeFileSync(statePath, JSON.stringify({ launched: true }));
      payload.writable = fs.readFileSync(statePath, "utf8").includes("launched");
      respond(request.id, {
        content: [{ type: "text", text: JSON.stringify(payload) }],
        structuredContent: payload,
        isError: false,
      });
    } catch (error) {
      respondError(request.id, -32603, error instanceof Error ? error.message : "runtime failure");
    }
    return;
  }
  if (request.method === "tools/call") {
    respondError(request.id, -32602, `unknown tool: ${String(request.params?.name ?? "")}`);
    return;
  }
  if (request.method === "ping") {
    respond(request.id, {});
    return;
  }
  respondError(request.id, -32601, `method not found: ${String(request.method ?? "")}`);
});
