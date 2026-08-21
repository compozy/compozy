import { AsyncLocalStorage } from "node:async_hooks";

import type { Effect } from "@compozy/extension-sdk";
import { createContext } from "react";
import type { ReactElement } from "react";

export interface NavigationController {
  push: (target: ReactElement) => void;
  pop: () => void;
  popToRoot: () => void;
}

export interface ViewRuntime {
  signal: AbortSignal;
  navigation: NavigationController;
  enqueueEffect: (effect: Effect) => void;
}

export const ViewRuntimeContext = createContext<ViewRuntime | null>(null);

const handlerRuntime = new AsyncLocalStorage<ViewRuntime>();

export async function runWithViewRuntime<T>(
  runtime: ViewRuntime,
  callback: () => Promise<T> | T
): Promise<T> {
  return await handlerRuntime.run(runtime, callback);
}

export function currentViewRuntime(): ViewRuntime {
  const runtime = handlerRuntime.getStore();
  if (!runtime) {
    throw new Error("view effect must run inside a CompozyOS view handler");
  }
  return runtime;
}
