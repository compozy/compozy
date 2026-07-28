import type { ProviderStateView } from "./provider-state";
import type { SettingsProviderEntry } from "../types";

function credentialReference(provider: SettingsProviderEntry): string {
  return (
    provider.credentials?.find(credential => credential.required)?.secret_ref ??
    provider.settings.credential_slots?.find(credential => credential.required)?.secret_ref ??
    ""
  ).trim();
}

/** Plain-language sign-in summary derived from both auth mode and real credential source. */
export function providerAuthSummary(provider: SettingsProviderEntry): string {
  const mode = provider.auth_status?.mode?.trim() || provider.settings.auth_mode?.trim();
  if (mode === "bound_secret") {
    const secretRef = credentialReference(provider);
    if (secretRef.startsWith("env:")) {
      const envName = secretRef.slice("env:".length).trim();
      return envName ? `Uses ${envName} from the environment` : "Uses an environment variable";
    }
    if (secretRef.startsWith("vault:")) return "Key stored in Vault";
    return "Credential binding needs attention";
  }
  if (mode === "none" || provider.auth_status?.state === "none") return "No sign-in needed";
  if (mode === "native_cli" || !mode) {
    switch (provider.auth_status?.state) {
      case "authenticated":
        return "Signed in with the local CLI";
      case "needs_login":
        return "Sign in with the local CLI";
      case "unknown":
      case undefined:
        return "Local sign-in not verified";
      default:
        return "Local sign-in unavailable";
    }
  }
  return "Authentication mode not recognized";
}

export function providerStatusCopy(
  state: ProviderStateView,
  modelCount: number
): { label: string; tone: ProviderStateView["tone"] } {
  if (state.label === "installed" && modelCount > 0) {
    return { label: `Ready · ${modelCount} models`, tone: state.tone };
  }
  return { label: state.display, tone: state.tone };
}
