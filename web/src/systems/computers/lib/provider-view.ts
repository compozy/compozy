import type { ProviderSummary } from "@/systems/providers";

import type { ComputerProviderAuthFixture } from "../mocks/remote-computers-types";

const KNOWN_LOGIN_COMMANDS: Readonly<Record<string, string>> = {
  claude: "claude login",
  codex: "codex login",
  copilot: "copilot login",
  cursor: "cursor login",
  opencode: "opencode auth login",
};

/**
 * Resolves the interactive login command to execute on the remote computer
 * when the operator clicks `Sign in on <name>` (`_user_stories.md` US-013, UT-091).
 */
export function providerLoginCommand(
  provider: Pick<ProviderSummary, "name" | "auth_status"> | ComputerProviderAuthFixture
): string {
  if ("loginCommand" in provider && provider.loginCommand && provider.loginCommand.trim() !== "") {
    return provider.loginCommand.trim();
  }
  const rawId = ("name" in provider && "auth_status" in provider ? provider.name : provider.id)
    .trim()
    .toLowerCase();
  const known = KNOWN_LOGIN_COMMANDS[rawId];
  if (known) return known;
  if ("auth_status" in provider) {
    const executable = provider.auth_status.login?.executable?.trim();
    if (executable) return `${executable} login`;
  }
  return `${rawId} login`;
}

/** Projects a live `/c/{id}/api/providers` summary into the Providers tab row model. */
export function toComputerProviderAuthView(provider: ProviderSummary): ComputerProviderAuthFixture {
  const state = provider.auth_status.state;
  const status: ComputerProviderAuthFixture["status"] =
    state === "authenticated" || state === "none"
      ? "ready"
      : state === "missing_cli"
        ? "not_installed"
        : "signed_out";
  const message = provider.auth_status.message?.trim();
  return {
    id: provider.name,
    name: provider.display_name?.trim() || provider.name,
    status,
    ...(status === "ready" && message ? { accountLabel: message } : {}),
    loginCommand: providerLoginCommand(provider),
  };
}
