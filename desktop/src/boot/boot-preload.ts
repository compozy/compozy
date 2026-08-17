import { contextBridge, ipcRenderer } from "electron";

import {
  BOOT_METHODS,
  validParams,
  validResponse,
  validState,
  type BootMethod,
} from "./boot-contract";

contextBridge.exposeInMainWorld("compozyBoot", {
  onState(listener: (value: unknown) => void): void {
    if (typeof listener !== "function") throw new TypeError("A boot-state listener is required.");
    ipcRenderer.on("boot:state", (_event, value: unknown) => {
      if (!validState(value)) throw new Error("The boot state is invalid.");
      listener(value);
    });
  },
  async invoke(method: string, params: unknown = {}): Promise<unknown> {
    if (!BOOT_METHODS.has(method as BootMethod))
      throw new Error("The boot action is not supported.");
    if (!validParams(method as BootMethod, params)) {
      throw new TypeError("Boot action parameters are invalid.");
    }
    const response: unknown = await ipcRenderer.invoke("boot:control", method, params);
    if (!validResponse(method as BootMethod, response)) {
      throw new Error("The boot action returned an invalid response.");
    }
    return response;
  },
});
