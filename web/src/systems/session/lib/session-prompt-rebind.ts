import type { SessionRuntimeEffective } from "../types";

function trimmed(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/**
 * Whether a prompt body names a provider or model other than the binding the
 * session ran when it was sent. The daemon applies that runtime before it
 * answers, so an accepted answer means the session's catalog rows changed.
 */
export function promptRebindsRuntime(
  body: BodyInit | null | undefined,
  before: SessionRuntimeEffective | null | undefined
): boolean {
  if (typeof body !== "string") return false;
  let runtime: unknown;
  try {
    runtime = (JSON.parse(body) as { runtime?: unknown }).runtime;
  } catch {
    return false;
  }
  if (typeof runtime !== "object" || runtime === null) return false;
  const provider = trimmed((runtime as { provider?: unknown }).provider);
  const model = trimmed((runtime as { model?: unknown }).model);
  if (provider === "") return false;
  return (
    provider !== trimmed(before?.provider) || (model !== "" && model !== trimmed(before?.model))
  );
}
