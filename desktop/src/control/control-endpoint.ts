import { readFile } from "node:fs/promises";

const WINDOWS_PIPE = /^\\\\\.\\pipe\\compozyos-[a-f0-9]{64}$/u;

/** Windows publishes a local named pipe in the private app.sock discovery file. */
export async function controlEndpoint(path: string): Promise<string> {
  if (process.platform !== "win32") return path;
  const endpoint = (await readFile(path, "utf8")).trim();
  if (!WINDOWS_PIPE.test(endpoint)) throw new Error("The app control pipe is invalid.");
  return endpoint;
}
