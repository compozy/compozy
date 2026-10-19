import { describe, expect, it } from "vitest";

import type { SettingsProviderEntry } from "../../types";
import {
  emptyProviderDraft,
  providerDraftFromEntry,
  providerDraftToRequest,
  withProviderAuthMode,
} from "../provider-draft";

function nativeACPEntry(overrides: Record<string, unknown> = {}): SettingsProviderEntry {
  return {
    name: "agy",
    default: false,
    command_available: true,
    settings: {
      command: "agy_acp_server.par --uid=",
      display_name: "Google Antigravity",
      harness: "acp",
      auth_mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      auth_acp_method: "oauth-personal",
      ...overrides,
    },
    auth_status: {
      mode: "native_cli",
      env_policy: "filtered",
      home_policy: "operator",
      state: "unknown",
      login: {
        configured: true,
        executable: "agy_acp_server.par",
        presence: "present",
        recommended_action: "inspect",
        source: "auth_acp_method",
      },
    },
    source_metadata: {
      available_targets: ["global-config"],
      effective_source: { kind: "builtin-provider", scope: "user" },
    },
  } as SettingsProviderEntry;
}

describe("providerDraft ACP sign-in method", () => {
  it("loads the protocol method from the entry", () => {
    expect(providerDraftFromEntry(nativeACPEntry()).auth_acp_method).toBe("oauth-personal");
  });

  it("preserves the method when an unrelated field is edited", () => {
    const draft = {
      ...providerDraftFromEntry(nativeACPEntry()),
      display_name: "Renamed",
    };
    expect(providerDraftToRequest(draft, "oauth-personal").settings?.auth_acp_method).toBe(
      "oauth-personal"
    );
  });

  it("submits a present-empty removal when the stored method is cleared", () => {
    const draft = { ...providerDraftFromEntry(nativeACPEntry()), auth_acp_method: "  " };
    const settings = providerDraftToRequest(draft, "oauth-personal").settings ?? {};
    expect("auth_acp_method" in settings).toBe(true);
    expect(settings.auth_acp_method).toBe("");
  });

  it("omits the method when it was never stored", () => {
    const draft = providerDraftFromEntry(nativeACPEntry({ auth_acp_method: undefined }));
    draft.auth_acp_method = "";
    expect("auth_acp_method" in (providerDraftToRequest(draft, "").settings ?? {})).toBe(false);
  });

  it("never submits a method for an inapplicable harness or auth mode", () => {
    const harnessDraft = {
      ...providerDraftFromEntry(nativeACPEntry()),
      harness: "pi_acp",
    };
    expect("auth_acp_method" in (providerDraftToRequest(harnessDraft, "").settings ?? {})).toBe(
      false
    );

    const modeDraft = withProviderAuthMode(providerDraftFromEntry(nativeACPEntry()), "none");
    expect(modeDraft.auth_acp_method).toBe("");
    expect("auth_acp_method" in (providerDraftToRequest(modeDraft, "").settings ?? {})).toBe(false);
  });

  it("removes a stored override when the draft becomes inapplicable", () => {
    const draft = {
      ...providerDraftFromEntry(nativeACPEntry()),
      harness: "pi_acp",
    };
    expect(providerDraftToRequest(draft, "oauth-personal").settings?.auth_acp_method).toBe("");
  });

  it("drops the method when auth ownership leaves native_cli", () => {
    const draft = providerDraftFromEntry(nativeACPEntry());
    expect(withProviderAuthMode(draft, "bound_secret").auth_acp_method).toBe("");
    expect(withProviderAuthMode(draft, "none").auth_acp_method).toBe("");
  });

  it("starts a create draft without a method", () => {
    expect(emptyProviderDraft().auth_acp_method).toBe("");
  });
});

describe("providerDraft command authorship", () => {
  it("omits an untouched inherited command from the request", () => {
    const draft = providerDraftFromEntry(nativeACPEntry({ command_inherited: true }));
    expect(draft.command_inherited).toBe(true);
    expect(providerDraftToRequest(draft).settings).not.toHaveProperty("command");
  });

  it("submits typed text when the operator edits an inherited command", () => {
    const draft = {
      ...providerDraftFromEntry(nativeACPEntry({ command_inherited: true })),
      command: "npx -y opencode-ai@latest acp",
      command_dirty: true,
    };
    expect(providerDraftToRequest(draft).settings?.command).toBe("npx -y opencode-ai@latest acp");
  });

  it("keeps an untouched authored command through an unrelated save", () => {
    const draft = { ...providerDraftFromEntry(nativeACPEntry()), display_name: "Renamed" };
    expect(providerDraftToRequest(draft).settings?.command).toBe("agy_acp_server.par --uid=");
  });

  it("omits a cleared authored command so the daemon returns to the default", () => {
    const draft = {
      ...providerDraftFromEntry(nativeACPEntry()),
      command: "",
      command_dirty: true,
    };
    expect(providerDraftToRequest(draft).settings).not.toHaveProperty("command");
  });
});
