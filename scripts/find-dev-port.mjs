import { createServer } from "node:net";

const [rawPreferredPort = "3000", mode = ""] = process.argv.slice(2);
const strict = mode === "--strict";

if (!/^\d+$/.test(rawPreferredPort) || (mode !== "" && !strict)) {
  console.error("usage: bun scripts/find-dev-port.mjs <preferred-port> [--strict]");
  process.exit(2);
}

const preferredPort = Number(rawPreferredPort);
if (!Number.isSafeInteger(preferredPort) || preferredPort < 1 || preferredPort > 65535) {
  console.error(`dev: invalid web port ${JSON.stringify(rawPreferredPort)}`);
  process.exit(2);
}

const lastPort = strict ? preferredPort : 65535;
let selectedPort;
for (let port = preferredPort; port <= lastPort; port += 1) {
  if (await portIsAvailable(port)) {
    selectedPort = port;
    break;
  }
}

if (selectedPort === undefined) {
  const detail = strict ? `port ${preferredPort} is already in use` : "no web port is available";
  console.error(`dev: ${detail}`);
  process.exit(1);
}

process.stdout.write(`${selectedPort}\n`);

async function portIsAvailable(port) {
  const ipv6Result = await attemptListen(port, "::");
  if (ipv6Result !== "unsupported") {
    return ipv6Result;
  }
  return (await attemptListen(port, "0.0.0.0")) === true;
}

function attemptListen(port, host) {
  return new Promise(resolve => {
    const server = createServer();
    server.unref();
    server.once("error", error => {
      if (error.code === "EAFNOSUPPORT" || error.code === "EADDRNOTAVAIL") {
        resolve("unsupported");
        return;
      }
      resolve(false);
    });
    server.listen(
      { host, port, exclusive: true, ipv6Only: host === "::" ? false : undefined },
      () => {
        server.close(() => resolve(true));
      }
    );
  });
}
