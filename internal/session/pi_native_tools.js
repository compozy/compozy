import { readFileSync, writeFileSync, renameSync } from "node:fs";
import { setTimeout } from "node:timers/promises";

// Versioned with Compozy; the MCP transport and process lifetime belong to Pi.
export default function (pi) {
  const config = JSON.parse(readFileSync(process.env.COMPOZY_PI_BRIDGE, "utf8"));
  const report = (code, count = 0) => {
    const temporary = config.status + ".tmp";
    try {
      writeFileSync(temporary, JSON.stringify({ nonce: config.nonce, code, count }), {
        mode: 0o600,
      });
      renameSync(temporary, config.status);
    } catch {
      // The daemon treats a missing status as unavailable; never crash the provider.
    }
  };
  if (
    [pi.registerMcpServer, pi.unregisterMcpServer, pi.getAllTools].some(
      api => typeof api !== "function"
    )
  ) {
    report("pi_native_tools_unsupported");
    return;
  }
  const registered = [];
  const disconnect = () => {
    for (const name of registered) pi.unregisterMcpServer(name);
  };
  try {
    for (const server of config.servers) {
      pi.registerMcpServer(server.name, server.config);
      registered.push(server.name);
    }
  } catch {
    disconnect();
    report("pi_native_tools_registration_failed");
    return;
  }
  let lifetime;
  pi.on("session_shutdown", () => lifetime?.abort());
  pi.on("session_start", () => {
    lifetime?.abort();
    lifetime = new AbortController();
    void check(lifetime.signal);
  });
  async function check(signal) {
    try {
      const deadline = Date.now() + config.timeout_ms;
      while (Date.now() < deadline) {
        const tools = pi
          .getAllTools()
          .filter(
            tool =>
              config.servers.some(server => tool.name.startsWith(`mcp__${server.name}__`)) &&
              tool.exposure !== "hidden"
          );
        if (
          config.servers.every(server =>
            tools.some(tool => tool.name.startsWith(`mcp__${server.name}__`))
          )
        ) {
          report("ready", tools.length);
          return;
        }
        await setTimeout(25, undefined, { signal });
      }
      disconnect();
      report("pi_native_tools_connection_failed");
    } catch {
      if (!signal.aborted) {
        disconnect();
        report("pi_native_tools_connection_failed");
      }
    }
  }
}
