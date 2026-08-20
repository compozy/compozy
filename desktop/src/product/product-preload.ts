import { contextBridge, ipcRenderer } from "electron";

import {
  PRODUCT_IPC_CHANNEL,
  isProductEvent,
  isProductMethod,
  validProductEventPayload,
  validProductParams,
  validProductResponse,
  type GlobalShortcutBinding,
  type GlobalShortcutRegistration,
  type ProductMethod,
} from "./product-contract";

async function invoke(
  method: ProductMethod,
  params: unknown
): Promise<GlobalShortcutRegistration[]> {
  if (!isProductMethod(method)) throw new Error("The product action is not supported.");
  if (!validProductParams(method, params)) {
    throw new TypeError("Product action parameters are invalid.");
  }
  const response: unknown = await ipcRenderer.invoke(PRODUCT_IPC_CHANNEL, method, params);
  if (!validProductResponse(method, response)) {
    throw new Error("The product action returned an invalid response.");
  }
  return response;
}

contextBridge.exposeInMainWorld("compozyShell", {
  platform: process.platform,
  on(event: string, listener: (value: unknown) => void): () => void {
    if (!isProductEvent(event)) {
      throw new Error("The product event is not supported.");
    }
    if (typeof listener !== "function")
      throw new TypeError("A product-event listener is required.");
    const wrapped = (_ipcEvent: unknown, value: unknown): void => {
      if (!validProductEventPayload(event, value)) {
        throw new Error("The product event payload is invalid.");
      }
      listener(value);
    };
    ipcRenderer.on(event, wrapped);
    return () => ipcRenderer.removeListener(event, wrapped);
  },
  globalShortcuts: {
    async sync(bindings: GlobalShortcutBinding[]): Promise<GlobalShortcutRegistration[]> {
      return await invoke("global_shortcuts.sync", { bindings });
    },
    async status(): Promise<GlobalShortcutRegistration[]> {
      return await invoke("global_shortcuts.status", {});
    },
  },
});
