import { randomUUID } from "node:crypto";

import type { Effect, ToastEffect } from "@compozy/extension-sdk";

import { currentViewRuntime } from "./runtime-context.js";

type EffectPayload = Omit<Effect, "id">;

export interface ToastOptions extends ToastEffect {}

export function queueViewEffect(effect: EffectPayload): string {
  const id = `effect_${randomUUID()}`;
  currentViewRuntime().enqueueEffect({ id, ...effect });
  return id;
}

export async function showToast(options: ToastOptions): Promise<void> {
  queueViewEffect({ toast: options });
}
