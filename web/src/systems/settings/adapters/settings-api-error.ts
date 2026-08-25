import type { OperationResponse } from "@/lib/api-contract";

type SettingsUpdateSkillsBadRequest = OperationResponse<"updateSettingsSkills", 400>;

/** Structured source-validation detail generated from the settings operation. */
export type SettingsErrorDetail = Extract<
  SettingsUpdateSkillsBadRequest,
  { error: object }
>["error"];

export class SettingsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly detail?: SettingsErrorDetail
  ) {
    super(message);
    this.name = "SettingsApiError";
  }
}

/**
 * Reads the `{error: {code, message, …}}` body the source-validation failures
 * use. The generic `{error: "text"}` branch has no source code and returns
 * undefined, so callers use the standard settings error formatter.
 */
export function settingsErrorDetail(error: unknown): SettingsErrorDetail | undefined {
  if (error == null || typeof error !== "object") return undefined;
  const body = Reflect.get(error, "error");
  if (body == null || typeof body !== "object") return undefined;
  const code = Reflect.get(body, "code");
  const message = Reflect.get(body, "message");
  if (typeof code !== "string" || typeof message !== "string") return undefined;
  const valid = Reflect.get(body, "valid");
  const suggestion = Reflect.get(body, "suggestion");
  const field = Reflect.get(body, "field");
  const path = Reflect.get(body, "path");
  const existingSource = Reflect.get(body, "existing_source");
  return {
    code,
    message,
    ...(Array.isArray(valid) ? { valid: valid.filter(item => typeof item === "string") } : {}),
    ...(typeof suggestion === "string" ? { suggestion } : {}),
    ...(typeof field === "string" ? { field } : {}),
    ...(typeof path === "string" ? { path } : {}),
    ...(typeof existingSource === "string" ? { existing_source: existingSource } : {}),
  };
}

export function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}
