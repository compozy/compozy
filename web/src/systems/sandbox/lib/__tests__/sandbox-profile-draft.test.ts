import { describe, expect, it } from "vitest";

import type { SettingsSandboxEntry } from "@/systems/settings";

import {
  emptySandboxDraft,
  sandboxEnvErrors,
  sandboxSecretEnvErrors,
  toSandboxDraft,
  toSandboxRequest,
  validateSandboxSecretEnvRef,
} from "../sandbox-profile-draft";

function makeEntry(profile: SettingsSandboxEntry["profile"]): SettingsSandboxEntry {
  return {
    name: "isolated-dev",
    profile,
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "global-config", scope: "global" },
    },
    workspace_usage_count: 2,
  };
}

describe("sandbox profile draft", () => {
  it("Should hydrate every nested profile key the editor now owns", () => {
    const draft = toSandboxDraft(
      makeEntry({
        backend: "daytona",
        sync_mode: "session-bidirectional",
        persistence: "transient",
        runtime_root: "/workspace",
        env: { LOG_LEVEL: "info" },
        secret_env: { GITHUB_TOKEN: "vault:sandbox/github" },
        network: { allow_outbound: true, allow_list: ["api.github.com"] },
        daytona: { api_url: "https://app.daytona.io/api", class: "medium" },
      })
    );

    expect(draft.env).toEqual([{ key: "LOG_LEVEL", value: "info" }]);
    expect(draft.secretEnv).toEqual([{ key: "GITHUB_TOKEN", value: "vault:sandbox/github" }]);
    expect(draft.network.allow_list).toEqual(["api.github.com"]);
    expect(draft.daytona.api_url).toBe("https://app.daytona.io/api");
  });

  it("Should round-trip a full profile through the replace body", () => {
    const profile = {
      backend: "daytona",
      sync_mode: "session-bidirectional",
      persistence: "transient",
      runtime_root: "/workspace",
      env: { LOG_LEVEL: "info" },
      secret_env: { GITHUB_TOKEN: "vault:sandbox/github" },
      network: { allow_public_ingress: true },
      daytona: { api_url: "https://app.daytona.io/api" },
    };

    expect(toSandboxRequest(toSandboxDraft(makeEntry(profile))).profile).toEqual(profile);
  });

  it("Should carry unmodelled profile keys through untouched", () => {
    const draft = toSandboxDraft(
      makeEntry({ backend: "local", future_key: "kept" } as SettingsSandboxEntry["profile"])
    );

    expect(toSandboxRequest(draft).profile).toMatchObject({ backend: "local", future_key: "kept" });
  });

  it("Should drop Daytona parameters once the backend leaves daytona", () => {
    const draft = toSandboxDraft(
      makeEntry({ backend: "daytona", daytona: { api_url: "https://app.daytona.io/api" } })
    );

    const request = toSandboxRequest({ ...draft, backend: "local" });

    expect(request.profile.backend).toBe("local");
    expect(request.profile).not.toHaveProperty("daytona");
  });

  it("Should omit blank env rows and empty collections instead of writing them", () => {
    const request = toSandboxRequest({
      ...emptySandboxDraft(),
      backend: "local",
      env: [
        { key: "", value: "ignored" },
        { key: "LOG_LEVEL", value: "info" },
      ],
      secretEnv: [{ key: "  ", value: "" }],
      network: { allow_list: ["  "] },
    });

    expect(request.profile.env).toEqual({ LOG_LEVEL: "info" });
    expect(request.profile).not.toHaveProperty("secret_env");
    expect(request.profile).not.toHaveProperty("network");
  });

  it("Should reject invalid and secret-like environment keys before serialization", () => {
    const draft = {
      ...emptySandboxDraft(),
      env: [
        { key: "9INVALID", value: "ignored" },
        { key: "GITHUB_TOKEN", value: "ignored" },
        { key: "LOG_LEVEL", value: "info" },
      ],
    };

    expect(sandboxEnvErrors(draft)).toEqual({
      0: "9INVALID is not a valid environment variable name.",
      1: "GITHUB_TOKEN must move secret-like values to secret environment.",
    });
    expect(toSandboxRequest(draft).profile.env).toEqual({ LOG_LEVEL: "info" });
  });

  it("Should omit Daytona network rules the provider cannot enforce", () => {
    const request = toSandboxRequest({
      ...emptySandboxDraft(),
      backend: "daytona",
      network: {
        required: true,
        allow_public_ingress: true,
        allow_outbound: true,
        allow_list: ["api.example.com"],
        deny_list: ["metadata.internal"],
      },
    });

    expect(request.profile.network).toEqual({ allow_public_ingress: true });
  });

  it("Should reject a literal secret_env value and keep it out of the replace body", () => {
    const draft = {
      ...emptySandboxDraft(),
      secretEnv: [{ key: "GITHUB_TOKEN", value: "ghp_liveTokenValue" }],
    };

    expect(sandboxSecretEnvErrors(draft)[0]).toContain("never a literal value");
    expect(toSandboxRequest(draft).profile).not.toHaveProperty("secret_env");
  });

  it("Should reject a vault ref outside the sandbox namespace", () => {
    const draft = {
      ...emptySandboxDraft(),
      secretEnv: [{ key: "TOKEN", value: "vault:providers/openai/api_key" }],
    };

    expect(sandboxSecretEnvErrors(draft)[0]).toContain("vault:sandbox/<path>");
  });

  it("Should reject a malformed env reference and an invalid variable name", () => {
    expect(validateSandboxSecretEnvRef({ key: "TOKEN", value: "env:not a name" })).toContain(
      "not a valid env reference"
    );
    expect(validateSandboxSecretEnvRef({ key: "9BAD", value: "env:TOKEN" })).toContain(
      "not a valid environment variable name"
    );
  });

  it("Should accept both reference forms the daemon allows", () => {
    expect(validateSandboxSecretEnvRef({ key: "TOKEN", value: "env:GITHUB_TOKEN" })).toBeNull();
    expect(
      validateSandboxSecretEnvRef({ key: "TOKEN", value: "vault:sandbox/github/token" })
    ).toBeNull();
    expect(validateSandboxSecretEnvRef({ key: "", value: "" })).toBeNull();
  });

  it("Should treat secret_env as references, never as plaintext to withhold", () => {
    const draft = toSandboxDraft(
      makeEntry({ backend: "local", secret_env: { TOKEN: "env:GITHUB_TOKEN" } })
    );

    // The daemon validates these as namespaced refs, so the editor round-trips
    // exactly what it displays.
    expect(draft.secretEnv).toEqual([{ key: "TOKEN", value: "env:GITHUB_TOKEN" }]);
    expect(toSandboxRequest(draft).profile.secret_env).toEqual({ TOKEN: "env:GITHUB_TOKEN" });
  });
});
