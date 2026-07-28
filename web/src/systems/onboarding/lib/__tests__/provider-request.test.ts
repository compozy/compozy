import { describe, expect, it } from "vitest";

import type { SettingsProviderRequest } from "@/systems/settings";

import { buildOnboardingProviderRequest } from "../provider-request";

const baseSettings: NonNullable<SettingsProviderRequest["settings"]> = {
  command: "npx claude",
  display_name: "Claude Code",
  models: {
    default: "claude-sonnet-5",
    curated: [
      {
        id: "claude-sonnet-5",
        display_name: "Claude Sonnet 5",
        reasoning_efforts: ["low", "medium", "high", "xhigh", "max"],
      },
    ],
  },
};

describe("buildOnboardingProviderRequest", () => {
  it("persists the chosen default model and reasoning while preserving existing settings", () => {
    const body = buildOnboardingProviderRequest(baseSettings, {
      model: "claude-opus-4-8",
      reasoning: "xhigh",
      authMode: "native_cli",
      envVar: "",
      apiKey: "",
      provider: "claude",
    });
    expect(body.settings?.command).toBe("npx claude");
    expect(body.settings?.models?.default).toBe("claude-opus-4-8");
    const curated = body.settings?.models?.curated ?? [];
    expect(curated).toEqual([{ id: "claude-sonnet-5" }, { id: "claude-opus-4-8" }]);
    expect(body.model_curation).toEqual({
      model_id: "claude-opus-4-8",
      default_effort: "xhigh",
    });
  });

  it("emits reasoning as explicit curation intent without copying merged enrichment", () => {
    const body = buildOnboardingProviderRequest(baseSettings, {
      model: "claude-sonnet-5",
      reasoning: "high",
      authMode: "native_cli",
      envVar: "",
      apiKey: "",
      provider: "claude",
    });
    const curated = body.settings?.models?.curated ?? [];
    expect(curated).toEqual([{ id: "claude-sonnet-5" }]);
    expect(body.model_curation).toEqual({
      model_id: "claude-sonnet-5",
      default_effort: "high",
    });
  });

  it("sets native_cli auth without credential slots or secrets", () => {
    const body = buildOnboardingProviderRequest(baseSettings, {
      model: "claude-opus-4-8",
      reasoning: "",
      authMode: "native_cli",
      envVar: "ANTHROPIC_API_KEY",
      apiKey: "sk-should-be-ignored",
      provider: "claude",
    });
    expect(body.settings?.auth_mode).toBe("native_cli");
    expect(body.settings?.credential_slots).toBeUndefined();
    expect(body.secrets).toBeUndefined();
  });

  it("clears stale credential slots when switching back to native_cli", () => {
    const body = buildOnboardingProviderRequest(
      {
        ...baseSettings,
        auth_mode: "bound_secret",
        credential_slots: [
          {
            name: "api_key",
            target_env: "ANTHROPIC_API_KEY",
            secret_ref: "env:ANTHROPIC_API_KEY",
            kind: "api_key",
            required: true,
          },
        ],
      },
      {
        model: "claude-opus-4-8",
        reasoning: "",
        authMode: "native_cli",
        envVar: "",
        apiKey: "sk-should-be-ignored",
        provider: "claude",
      }
    );
    expect(body.settings?.auth_mode).toBe("native_cli");
    expect(body.settings).not.toHaveProperty("credential_slots");
    expect(body.secrets).toBeUndefined();
  });

  it("binds an env-var reference without a secret value when no key is provided", () => {
    const body = buildOnboardingProviderRequest(baseSettings, {
      model: "claude-opus-4-8",
      reasoning: "",
      authMode: "bound_secret",
      envVar: "ANTHROPIC_API_KEY",
      apiKey: "",
      provider: "claude",
    });
    expect(body.settings?.auth_mode).toBe("bound_secret");
    expect(body.settings?.credential_slots?.[0]?.secret_ref).toBe("env:ANTHROPIC_API_KEY");
    expect(body.secrets).toBeUndefined();
  });

  it("rejects bound_secret when no target environment variable is known", () => {
    expect(() =>
      buildOnboardingProviderRequest(baseSettings, {
        model: "claude-opus-4-8",
        reasoning: "",
        authMode: "bound_secret",
        envVar: "",
        apiKey: "sk-real-key",
        provider: "claude",
      })
    ).toThrow("Enter the environment variable the provider expects.");
  });

  it("reuses an existing credential slot target when the operator keeps it blank", () => {
    const body = buildOnboardingProviderRequest(
      {
        ...baseSettings,
        credential_slots: [
          {
            name: "api_key",
            target_env: "ANTHROPIC_API_KEY",
            secret_ref: "env:ANTHROPIC_API_KEY",
            kind: "api_key",
            required: true,
          },
        ],
      },
      {
        model: "claude-opus-4-8",
        reasoning: "",
        authMode: "bound_secret",
        envVar: "",
        apiKey: "",
        provider: "claude",
      }
    );
    expect(body.settings?.credential_slots?.[0]?.target_env).toBe("ANTHROPIC_API_KEY");
    expect(body.settings?.credential_slots?.[0]?.secret_ref).toBe("env:ANTHROPIC_API_KEY");
  });

  it("stores a provided API key as a vault-backed secret", () => {
    const body = buildOnboardingProviderRequest(baseSettings, {
      model: "claude-opus-4-8",
      reasoning: "",
      authMode: "bound_secret",
      envVar: "ANTHROPIC_API_KEY",
      apiKey: "sk-real-key",
      provider: "claude",
    });
    expect(body.settings?.credential_slots?.[0]?.secret_ref).toBe("vault:providers/claude/api_key");
    expect(body.secrets).toEqual([
      {
        name: "api_key",
        secret_ref: "vault:providers/claude/api_key",
        kind: "api_key",
        value: "sk-real-key",
      },
    ]);
  });
});
