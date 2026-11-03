import type { AgentAvatarAppearance, SymbolBackground, SymbolValue } from "@compozy/ui";

/** The symbol pane's starting point for an agent that still wears a generated face. */
const DEFAULT_ICON = "compass";

/** What the symbol picker shows for an appearance. */
export function agentSymbolOf(appearance: AgentAvatarAppearance): SymbolValue {
  if (appearance.avatar === "emoji") return { kind: "emoji", value: appearance.emoji };
  return { kind: "icon", value: appearance.avatar === "icon" ? appearance.icon : DEFAULT_ICON };
}

/** The plate choice of an appearance; a generated face and an unset value keep the plate. */
export function agentSymbolBackgroundOf(appearance: AgentAvatarAppearance): SymbolBackground {
  return appearance.avatar !== "generated" && appearance.background === "none" ? "none" : "plate";
}

/** The appearance a symbol pick saves; the default plate is left unset, as older agents store it. */
export function agentSymbolAppearance(
  symbol: SymbolValue,
  color: string,
  background: SymbolBackground
): AgentAvatarAppearance {
  const plate = background === "none" ? { background } : {};
  return symbol.kind === "emoji"
    ? { avatar: "emoji", emoji: symbol.value, color, ...plate }
    : { avatar: "icon", icon: symbol.value, color, ...plate };
}
