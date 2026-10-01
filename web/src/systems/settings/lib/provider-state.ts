import type { PillTone } from "@compozy/ui";

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
        hint: slot ? `Add a value for ${slot} to continue.` : "A required key is missing.",
        cta: { label: "Add key", intent: "configure" },
      };
    }
    case "binary-missing":
      // Absence is a fact, not a warning: the amber stays for broken setups.
      return {
        tone: "neutral",
        label,
        display: "Not installed",
        hint: "The app for this provider isn't installed on this computer yet.",
        cta: { label: "Edit settings", intent: "edit" },
      };
    case "needs-sign-in":
      return {
        tone: "warning",
        label,
        display: "Needs sign-in",
        hint: provider.auth_status?.message?.trim() || "Sign in before starting a session.",
        cta: { label: "Sign in", intent: "configure" },
      };
    case "auth-unknown":
      return {
        tone: "info",
        label,
        display: "Sign-in unverified",
        hint:
          provider.auth_status?.message?.trim() || "CompozyOS couldn't confirm you're signed in.",
        cta: { label: "Check sign-in", intent: "edit" },
      };
    case "auth-unavailable":
      return {
        tone: "danger",
        label,
        display: "Sign-in unavailable",
        hint: provider.auth_status?.message?.trim() || "CompozyOS couldn't check the sign-in.",
        cta: { label: "Check sign-in", intent: "edit" },
      };
  }
}
