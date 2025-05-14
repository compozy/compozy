import { NatsClient } from "../src/nats_client.ts";
export const fixturesPath = new URL("../fixtures", import.meta.url).pathname;
export const echoToolPath = `${fixturesPath}/echo_tool.ts`;
export const weatherToolPath = `${fixturesPath}/weather_tool.ts`;

export let capturedOutput: string[] = [];
const originalConsoleError = console.error;
const originalConsoleLog = console.log;

export function setupCapture() {
  capturedOutput = [];
  console.error = (message: string) => {
    capturedOutput.push(message);
  };
  console.log = (message: string) => {
    capturedOutput.push(message);
  };
}

export function restoreConsole() {
  console.error = originalConsoleError;
  console.log = originalConsoleLog;
}

export function getRdnNamespace() {
  return `test_${crypto.randomUUID().replace(/-/g, "")}`;
}

export async function setupNats(options: {
  verbose?: boolean;
  namespace?: string;
  execId?: string;
  serverUrl?: string;
} = {
  verbose: false,
  namespace: getRdnNamespace(),
  execId: undefined,
  serverUrl: "nats://localhost:4222",
}) {
  const testNamespace = options.namespace || getRdnNamespace();
  const natsClient = new NatsClient({
    verbose: options.verbose,
    namespace: testNamespace,
    serverUrl: options.serverUrl,
    ...(options.execId && { execId: options.execId }),
  });
  await natsClient.connect();
  return natsClient;
}
