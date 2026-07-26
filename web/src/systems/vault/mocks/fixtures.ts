import type { VaultSecret, VaultSecretsResponse } from "@/systems/vault";

export const vaultSecretFixtures: VaultSecret[] = [
  {
    ref: "vault:sessions/sess_frontend_launch_qa/openai",
    namespace: "sessions",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T08:30:00Z",
    updated_at: "2026-04-17T11:45:00Z",
  },
  {
    ref: "vault:providers/claude/default",
    namespace: "providers",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T07:10:00Z",
    updated_at: "2026-04-17T10:20:00Z",
  },
  {
    ref: "vault:bridges/slack/launch-room",
    namespace: "bridges",
    kind: "bot_token",
    present: true,
    created_at: "2026-04-16T19:05:00Z",
    updated_at: "2026-04-17T09:00:00Z",
  },
];

export const vaultSecretsResponseFixture: VaultSecretsResponse = {
  secrets: vaultSecretFixtures,
};
