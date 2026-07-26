import type { PillTone } from "@agh/ui";

import type { SettingsProviderEntry } from "../types";

export type ProviderStateLabel =
  | "installed"
  | "binary-missing"
  | "unconfigured"
  | "needs-sign-in"
  | "auth-unknown"
  | "auth-unavailable";

export type ProviderStateIntent = "edit" | "configure";

export interface ProviderStateView {
  tone: PillTone;
  label: ProviderStateLabel;
  display: string;
  hint: string | null;
  cta: { label: string; intent: ProviderStateIntent };
}

export function providerCredentialsConfigured(provider: SettingsProviderEntry): boolean {
  const credentials = provider.credentials ?? [];
  return credentials.every(credential => !credential.required || credential.present);
}

function authMode(provider: SettingsProviderEntry): string {
  return provider.auth_status?.mode?.trim() || provider.settings.auth_mode?.trim() || "native_cli";
}

export function deriveProviderStateLabel(provider: SettingsProviderEntry): ProviderStateLabel {
  const authState = provider.auth_status?.state?.trim() || "unknown";
  if (!provider.command_available || authState === "missing_cli") return "binary-missing";
  if (!providerCredentialsConfigured(provider) || authState === "missing_credential") {
    return "unconfigured";
  }
  if (authMode(provider) === "none" || authState === "none" || authState === "authenticated") {
    return "installed";
  }
  if (authState === "needs_login") return "needs-sign-in";
  if (authState === "unknown") return "auth-unknown";
  return "auth-unavailable";
}

function firstMissingRequiredSlot(provider: SettingsProviderEntry): string | null {
  const missing = (provider.credentials ?? []).find(
    credential => credential.required && !credential.present
  );
  return missing?.target_env || missing?.name || null;
}

export function getProviderStateView(provider: SettingsProviderEntry): ProviderStateView {
  const label = deriveProviderStateLabel(provider);
  switch (label) {
    case "installed":
      return {
        tone: "success",
        label,
        display: "Ready",
        hint: null,
        cta: { label: "Edit settings", intent: "edit" },
      };
    case "unconfigured": {
      const slot = firstMissingRequiredSlot(provider);
      return {
        tone: "warning",
        label,
        display: "Needs setup",
        hint: slot ? `Bind ${slot} to continue.` : "Required credential is missing.",
        cta: { label: "Configure credentials", intent: "configure" },
      };
    }
    case "binary-missing": {
      const command = provider.settings.command?.trim() || provider.name;
      return {
        tone: "warning",
        label,
        display: "Not installed",
        hint: `${command} not found on PATH.`,
        cta: { label: "Edit settings", intent: "edit" },
      };
    }
    case "needs-sign-in":
      return {
        tone: "warning",
        label,
        display: "Needs sign-in",
        hint: provider.auth_status?.message?.trim() || "Sign in before starting a session.",
        cta: { label: "Review sign-in", intent: "configure" },
      };
    case "auth-unknown":
      return {
        tone: "info",
        label,
        display: "Sign-in unverified",
        hint: provider.auth_status?.message?.trim() || "AGH has not verified the local sign-in.",
        cta: { label: "Inspect sign-in", intent: "edit" },
      };
    case "auth-unavailable":
      return {
        tone: "danger",
        label,
        display: "Sign-in unavailable",
        hint: provider.auth_status?.message?.trim() || "The authentication check failed.",
        cta: { label: "Inspect sign-in", intent: "edit" },
      };
  }
}
