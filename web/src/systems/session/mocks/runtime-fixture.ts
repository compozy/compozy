import type { SessionPayload } from "@/systems/session/types";

const DEFAULT_ACP_CAPS = {
  prompt_audio: false,
  prompt_embedded_context: false,
  prompt_image: false,
  supports_load_session: false,
} as const;

export function sessionRuntime(
  provider: string,
  acpCaps?: Partial<NonNullable<SessionPayload["runtime"]["acp_caps"]>>
): SessionPayload["runtime"] {
  return {
    status: "ready",
    transition: "initial_bind",
    effective: { provider },
    selection_revision: 0,
    ...(acpCaps
      ? {
          acp_caps: {
            ...DEFAULT_ACP_CAPS,
            ...acpCaps,
          },
        }
      : {}),
  };
}
