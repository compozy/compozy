import readline from "node:readline";
import fs from "node:fs";
import path from "node:path";

const lines = readline.createInterface({ input: process.stdin });

const respond = (id, result) => {
  process.stdout.write(`${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`);
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
    const statePath = process.argv[2];
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
    return;
  }
  if (request.method === "ping") {
    respond(request.id, {});
    return;
  }
  respond(request.id, {});
});
