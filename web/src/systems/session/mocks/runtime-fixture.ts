import type { SessionPayload } from "@/systems/session/types";

const DEFAULT_ACP_CAPS = {
  prompt_audio: false,
  prompt_embedded_context: false,
  prompt_image: false,
  supports_fork_session: false,
  supports_load_session: false,
  supports_resume_session: false,
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

const FIXTURE_MODELS: Record<string, { model: string; reasoning_effort: string }> = {
  claude: { model: "claude-opus-4-1", reasoning_effort: "high" },
  codex: { model: "gpt-5", reasoning_effort: "medium" },
};

/** A bound runtime that also records the model and effort the gallery rows name on line 2. */
export function sessionRuntimeWithModel(provider: string): SessionPayload["runtime"] {
  const runtime = sessionRuntime(provider);
  const model = FIXTURE_MODELS[provider];
  return model ? { ...runtime, effective: { provider, ...model } } : runtime;
}
