import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { ProviderDetailDialog } from "../provider-detail-dialog";
import {
  emptyProviderDraft,
  providerDraftFromEntry,
  withProviderAuthMode,
} from "@/systems/settings/lib/provider-draft";
import type { ProviderDraft, SettingsProviderEntry } from "@/systems/settings";

const nativeProvider: SettingsProviderEntry = {
  name: "codex",
  default: false,
  command_available: true,
  settings: {
    command: "npx -y @agentclientprotocol/codex-acp@latest",
    display_name: "Codex",
    models: { default: "gpt-5.6-sol", curated: [{ id: "gpt-5.6-sol" }] },
    harness: "acp",
    auth_mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    auth_status_command: "codex auth status",
    auth_login_command: "codex login",
  },
  auth_status: {
    mode: "native_cli",
    env_policy: "filtered",
    home_policy: "operator",
    state: "authenticated",
  },
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "global-config", scope: "global" },
  },
};

const boundSecretProvider: SettingsProviderEntry = {
  name: "openrouter",
  default: false,
  command_available: true,
  settings: {
    command: "npx -y pi-acp@latest",
    display_name: "OpenRouter",
    models: { default: "openai/gpt-5.4", curated: [{ id: "openai/gpt-5.4" }] },
    harness: "pi_acp",
    runtime_provider: "openrouter",
    auth_mode: "bound_secret",
    env_policy: "filtered",
    home_policy: "operator",
    credential_slots: [
      {
        name: "api_key",
        target_env: "OPENROUTER_API_KEY",
        secret_ref: "vault:providers/openrouter/api-key",
        kind: "api_key",
        required: true,
      },
    ],
  },
  credentials: [
    {
      name: "api_key",
      target_env: "OPENROUTER_API_KEY",
      secret_ref: "vault:providers/openrouter/api-key",
      present: true,
      required: true,
    },
  ],
  source_metadata: {
    available_targets: ["global-config"],
    effective_source: { kind: "global-config", scope: "global" },
  },
};

interface HarnessProps {
  mode: "inspect" | "edit" | "create";
  entry?: SettingsProviderEntry | null;
  initialDraft?: ProviderDraft;
  onSave?: () => void;
}

/**
 * Mirrors the page hook's inspector state machine so tab and footer entry
 * points are exercised through the transition they actually drive.
 */
function Harness({
  mode: initialMode,
  entry = null,
  initialDraft,
  onSave = vi.fn(),
}: HarnessProps) {
  const [mode, setMode] = useState(initialMode);
  const [draft, setDraft] = useState<ProviderDraft>(initialDraft ?? emptyProviderDraft());
  return (
    <ProviderDetailDialog
      canSave
      draft={mode === "inspect" ? null : draft}
      entry={entry}
      error={null}
      isDeleting={false}
      isSaving={false}
      mode={mode}
      onCancelEdit={() => setMode(current => (current === "edit" ? "inspect" : current))}
      onDraftChange={updater => setDraft(current => updater(current))}
      onOpenChange={vi.fn()}
      onRefreshCatalog={vi.fn()}
      onRequestDelete={vi.fn()}
      onSave={onSave}
      onSwitchToEdit={() => {
        setMode(current => {
          if (current !== "inspect" || !entry) return current;
          setDraft(providerDraftFromEntry(entry));
          return "edit";
        });
      }}
      open
      warnings={undefined}
    />
  );
}

function renderDialog(props: HarnessProps) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return render(<Harness {...props} />, { wrapper });
}

function dialog() {
  return screen.getByTestId("provider-detail-dialog");
}

describe("ProviderDetailDialog", () => {
  it("Should render the entity header with an icon well and no runtime selector on create", () => {
    renderDialog({ mode: "create" });

    const header = dialog().querySelector('[data-slot="entity-dialog-header"]');
    expect(header).not.toBeNull();
    expect(header?.querySelector('[data-slot="entity-dialog-header-icon"]')).not.toBeNull();
    expect(screen.getByTestId("provider-detail-title")).toHaveTextContent("Create provider");
    expect(within(dialog()).getByText("Settings · Provider")).toBeInTheDocument();
    // A provider surface configures a provider; it never chooses a runtime.
    expect(dialog().querySelector('[data-slot="runtime-selector"]')).toBeNull();
    expect(screen.queryByTestId("runtime-selector-trigger")).not.toBeInTheDocument();
  });

  it("Should hide credential controls under native_cli and show the provider-owned login command", () => {
    renderDialog({ mode: "create" });

    expect(
      screen.queryByTestId("settings-providers-editor-credential-slots")
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("settings-providers-editor-credential-slot-0")
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-providers-editor-auth-login-command-input")).toBeVisible();
    expect(screen.getByTestId("settings-providers-editor-auth-status-command-input")).toBeVisible();
  });

  it("Should reveal credential slots only after auth ownership becomes bound_secret", async () => {
    const user = userEvent.setup();
    renderDialog({ mode: "create" });

    await user.click(screen.getByTestId("settings-providers-editor-auth-mode-bound_secret"));

    expect(screen.getByTestId("settings-providers-editor-credential-slot-0")).toBeVisible();
    expect(
      screen.getByTestId("settings-providers-editor-credential-slot-0-target-env-input")
    ).toBeVisible();
    // Leaving the mode takes the slots with it — the daemon rejects them elsewhere.
    await user.click(screen.getByTestId("settings-providers-editor-auth-mode-native_cli"));
    expect(
      screen.queryByTestId("settings-providers-editor-credential-slot-0")
    ).not.toBeInTheDocument();
  });

  it("Should offer a write input only for a vault-backed credential ref", async () => {
    const user = userEvent.setup();
    renderDialog({
      mode: "create",
      initialDraft: withProviderAuthMode(emptyProviderDraft(), "bound_secret"),
    });

    const secretRef = screen.getByTestId(
      "settings-providers-editor-credential-slot-0-secret-ref-input"
    );
    await user.type(secretRef, "env:OPENAI_API_KEY");
    expect(
      screen.getByTestId("settings-providers-editor-credential-slot-0-value-unavailable")
    ).toBeVisible();
    expect(screen.queryByLabelText("Credential value")).not.toBeInTheDocument();

    await user.clear(secretRef);
    await user.type(secretRef, "vault:providers/openai/api-key");
    expect(screen.getByLabelText("Credential value")).toBeVisible();
  });

  it("Should keep a stored credential rotate-only on edit", async () => {
    const user = userEvent.setup();
    renderDialog({
      mode: "edit",
      entry: boundSecretProvider,
      initialDraft: providerDraftFromEntry(boundSecretProvider),
    });

    const presence = screen.getByTestId(
      "settings-providers-editor-credential-slot-0-value-presence"
    );
    expect(presence).toHaveTextContent("vault:providers/openrouter/api-key → OPENROUTER_API_KEY");
    expect(screen.queryByLabelText("Credential value")).not.toBeInTheDocument();

    await user.click(
      screen.getByTestId("settings-providers-editor-credential-slot-0-value-replace")
    );
    const input = screen.getByLabelText("Credential value");
    expect(input).toHaveValue("");
    expect(input).toHaveAttribute("type", "password");
  });

  it("Should render the provider name as immutable identity on edit", () => {
    renderDialog({
      mode: "edit",
      entry: nativeProvider,
      initialDraft: providerDraftFromEntry(nativeProvider),
    });

    const identity = screen.getByTestId("settings-providers-editor-identity");
    expect(identity).toHaveTextContent("codex");
    expect(screen.queryByTestId("settings-providers-editor-name-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("provider-detail-title")).toHaveTextContent("Edit codex");
  });

  it("Should keep runtime and model fields in the Advanced tier", async () => {
    const user = userEvent.setup();
    renderDialog({ mode: "create" });

    expect(screen.queryByTestId("settings-providers-editor-model-input")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("settings-providers-editor-mode-advanced"));

    expect(screen.getByTestId("settings-providers-editor-model-input")).toBeVisible();
    expect(screen.getByTestId("settings-providers-editor-harness-input")).toBeVisible();
    expect(screen.getByTestId("settings-providers-editor-home-policy-input")).toBeVisible();
    // Advanced appends; the required basics never disappear.
    expect(screen.getByTestId("settings-providers-editor-name-input")).toBeVisible();
  });

  it("Should mark runtime provider as required only for the pi_acp harness", async () => {
    const user = userEvent.setup();
    renderDialog({ mode: "create" });

    await user.click(screen.getByTestId("settings-providers-editor-mode-advanced"));
    const runtimeProvider = screen.getByTestId("settings-providers-editor-runtime-provider");
    expect(within(runtimeProvider).queryByText("required")).not.toBeInTheDocument();

    await user.selectOptions(
      screen.getByTestId("settings-providers-editor-harness-input"),
      "pi_acp"
    );

    expect(within(runtimeProvider).getByText("required")).toBeVisible();
  });

  it("Should open the edit form from the Configure tab without losing inspect", async () => {
    const user = userEvent.setup();
    renderDialog({ mode: "inspect", entry: nativeProvider });

    expect(screen.getByTestId("provider-detail-title")).toHaveTextContent("codex");
    expect(screen.queryByTestId("settings-providers-editor-mode-advanced")).not.toBeInTheDocument();
    expect(screen.getByTestId("inspect-auth-mode")).toHaveTextContent("native_cli");

    await user.click(screen.getByTestId("provider-detail-tab-configure"));

    expect(screen.getByTestId("settings-providers-editor-basics")).toBeVisible();
    expect(screen.getByTestId("provider-detail-title")).toHaveTextContent("Edit codex");

    await user.click(screen.getByTestId("provider-detail-tab-overview"));
    expect(screen.getByTestId("inspect-auth-mode")).toBeVisible();
    expect(screen.queryByTestId("settings-providers-editor-basics")).not.toBeInTheDocument();
  });

  it("Should reopen on the Simple tier after an Advanced edit of another provider", async () => {
    const user = userEvent.setup();
    const { rerender } = renderDialog({
      mode: "edit",
      entry: nativeProvider,
      initialDraft: providerDraftFromEntry(nativeProvider),
    });

    await user.click(screen.getByTestId("settings-providers-editor-mode-advanced"));
    expect(screen.getByTestId("settings-providers-editor-model-input")).toBeVisible();

    // The page keeps this dialog mounted between openings.
    rerender(
      <Harness
        entry={boundSecretProvider}
        initialDraft={providerDraftFromEntry(boundSecretProvider)}
        mode="edit"
      />
    );

    expect(screen.queryByTestId("settings-providers-editor-model-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-providers-editor-basics")).toBeVisible();
  });

  it("Should open the edit form from the inspect footer action", async () => {
    const user = userEvent.setup();
    renderDialog({ mode: "inspect", entry: nativeProvider });

    await user.click(screen.getByTestId("provider-detail-edit"));

    expect(screen.getByTestId("settings-providers-editor-basics")).toBeVisible();
    expect(screen.getByTestId("provider-detail-save")).toHaveTextContent("Save provider");
  });
});
