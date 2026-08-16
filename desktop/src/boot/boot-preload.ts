import { contextBridge, ipcRenderer } from "electron";

type BootMethod =
  | "retry"
  | "diagnose"
  | "copy_diagnostics"
  | "export_diagnostics"
  | "open_logs"
  | "quit";

const METHODS = new Set<BootMethod>([
  "retry",
  "diagnose",
  "copy_diagnostics",
  "export_diagnostics",
  "open_logs",
  "quit",
]);
const STATES = new Set([
  "resolving",
  "provisioning",
  "starting",
  "attaching",
  "product",
  "updating",
  "disconnected",
  "skew",
  "error",
]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function validState(value: unknown): boolean {
  if (!isRecord(value) || typeof value.state !== "string" || !STATES.has(value.state)) return false;
  if (value.error !== undefined) {
    if (!isRecord(value.error)) return false;
    if (
      typeof value.error.code !== "string" ||
      typeof value.error.safe_message !== "string" ||
      typeof value.error.log_path !== "string"
    ) {
      return false;
    }
  }
  return true;
}

function validResponse(method: BootMethod, value: unknown): boolean {
  if (!isRecord(value)) return false;
  switch (method) {
    case "retry":
      return value.ok === true || value.accepted === true;
    case "diagnose":
      return value.schema_version === 1;
    case "copy_diagnostics":
      return value.copied === true;
    case "export_diagnostics":
      return typeof value.bundle_path === "string" && typeof value.bytes === "number";
    case "open_logs":
      return value.opened === true;
    case "quit":
      return value.quitting === true;
  }
}

contextBridge.exposeInMainWorld("compozyBoot", {
  onState(listener: (value: unknown) => void): void {
    if (typeof listener !== "function") throw new TypeError("A boot-state listener is required.");
    ipcRenderer.on("boot:state", (_event, value: unknown) => {
      if (!validState(value)) throw new Error("The boot state is invalid.");
      listener(value);
    });
  },
  async invoke(method: string, params: unknown = {}): Promise<unknown> {
    if (!METHODS.has(method as BootMethod)) throw new Error("The boot action is not supported.");
    if (!params || typeof params !== "object" || Array.isArray(params)) {
      throw new TypeError("Boot action parameters must be an object.");
    }
    const response: unknown = await ipcRenderer.invoke("boot:control", method, params);
    if (!validResponse(method as BootMethod, response)) {
      throw new Error("The boot action returned an invalid response.");
    }
    return response;
  },
});
