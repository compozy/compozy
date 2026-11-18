"use client";

import { useSyncExternalStore } from "react";
import { detectVisitorPlatform, type VisitorPlatform } from "@/lib/release/desktop-downloads";

/** The server and first client render agree on this, so hydration never mismatches. */
const SERVER_PLATFORM: VisitorPlatform = "mac";

type UserAgentDataLike = {
  getHighEntropyValues?: (hints: string[]) => Promise<{ architecture?: string }>;
};

let snapshot: VisitorPlatform | null = null;
let refining = false;
const listeners = new Set<() => void>();

function gpuRenderer(): string | undefined {
  try {
    const gl = document.createElement("canvas").getContext("webgl");
    const info = gl?.getExtension("WEBGL_debug_renderer_info");
    if (!gl || !info) return undefined;
    const renderer: unknown = gl.getParameter(info.UNMASKED_RENDERER_WEBGL);
    return typeof renderer === "string" ? renderer : undefined;
  } catch {
    return undefined;
  }
}

function signals(architecture?: string) {
  return {
    userAgent: navigator.userAgent,
    maxTouchPoints: navigator.maxTouchPoints,
    architecture,
    gpuRenderer: gpuRenderer(),
  };
}

function publish(next: VisitorPlatform) {
  if (next === snapshot) return;
  snapshot = next;
  for (const listener of listeners) listener();
}

/** Chromium hides the CPU in its user agent; its client hints still report it. */
function refineMacArchitecture() {
  if (refining || getSnapshot() !== "mac") return;
  refining = true;
  const uaData = (navigator as Navigator & { userAgentData?: UserAgentDataLike }).userAgentData;
  uaData
    ?.getHighEntropyValues?.(["architecture"])
    .then(({ architecture }) => publish(detectVisitorPlatform(signals(architecture))))
    .catch(() => undefined);
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  refineMacArchitecture();
  return () => listeners.delete(listener);
}

function getSnapshot(): VisitorPlatform {
  snapshot ??= detectVisitorPlatform(signals());
  return snapshot;
}

function getServerSnapshot(): VisitorPlatform {
  return SERVER_PLATFORM;
}

export function useVisitorPlatform(): VisitorPlatform {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
