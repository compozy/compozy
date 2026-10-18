import { KeyRound, ShieldOff, TerminalSquare } from "lucide-react";

import { Alert, AlertDescription, Eyebrow, FormSection, Input, Pill, RadioCard } from "@compozy/ui";

import { providerAuthStateLabel } from "../lib/provider-copy";
import { withProviderAuthMode } from "../lib/provider-draft";
import type { ProviderAuthMode, ProviderDraft, SettingsProviderEntry } from "../types";
import type { ProviderDraftChange } from "./provider-edit-form";
import { ProviderCredentialFields } from "./provider-edit-form-credential-fields";
import { ProviderLoginDescriptorView } from "./provider-login-descriptor";
import { ModalSettingsFieldRow } from "./settings-field-row";

interface ProviderAuthFieldsProps {
  mode: "create" | "edit";
  draft: ProviderDraft;
  entry: SettingsProviderEntry | null;
  onChange: ProviderDraftChange;
}

const AUTH_CARDS: ReadonlyArray<{
  value: ProviderAuthMode;
  title: string;
  description: string;
  badge: string;
  icon: typeof KeyRound;
  testId: string;
}> = [
  {
    value: "native_cli",
    title: "Provider's own sign-in",
    description: "You sign in with the provider's app. CompozyOS never asks for keys.",
    badge: "Provider-owned",
    icon: TerminalSquare,
    testId: "settings-providers-editor-auth-mode-native_cli",
  },
  {
    value: "bound_secret",
    title: "Key saved in CompozyOS",
    description: "CompozyOS passes your saved key to the provider when it starts.",
    badge: "CompozyOS-managed",
    icon: KeyRound,
    testId: "settings-providers-editor-auth-mode-bound_secret",
  },
  {
    value: "none",
    title: "No sign-in",
    description: "The provider starts without any keys from CompozyOS.",
    badge: "Unauthenticated",
    icon: ShieldOff,
    testId: "settings-providers-editor-auth-mode-none",
  },
];

/**
 * Auth ownership and everything it governs.
 *
 * The mode is a security boundary, not a disclosure: CompozyOS may offer credential
 * inputs only under `bound_secret`, and under `native_cli` it may do no more
 * than surface its safe login descriptor (`internal/CLAUDE.md` § Provider
 * auth boundary). The gate is therefore mount/unmount, never
 * disabled fields — a disabled credential input still says CompozyOS wants the key.
 */
export function ProviderAuthFields({ mode, draft, entry, onChange }: ProviderAuthFieldsProps) {
  return (
    <FormSection
      data-testid="settings-providers-editor-auth"
      description="CompozyOS asks for keys only when it manages them for you."
      title="Who owns authentication?"
    >
      <div
        aria-label="Authentication ownership"
        className="grid gap-2 sm:grid-cols-3"
        data-testid="settings-providers-editor-auth-mode"
        role="radiogroup"
      >
        {AUTH_CARDS.map(card => (
          <RadioCard
            data-testid={card.testId}
            description={
              <span className="flex flex-col items-start gap-1.5">
                {card.description}
                <Pill size="xs" tone={card.value === "bound_secret" ? "info" : "neutral"}>
                  {card.badge}
                </Pill>
              </span>
            }
            icon={card.icon}
            key={card.value}
            onSelect={() => onChange(current => withProviderAuthMode(current, card.value))}
            selected={draft.auth_mode === card.value}
            title={card.title}
            titleClassName="min-w-0 flex-1 truncate"
          />
        ))}
      </div>

      {draft.auth_mode === "native_cli" ? (
        <ProviderNativeAuthFields
          draft={draft}
          entry={mode === "edit" ? entry : null}
          onChange={onChange}
        />
      ) : null}

      {draft.auth_mode === "bound_secret" ? (
        <ProviderCredentialFields draft={draft} entry={entry} mode={mode} onChange={onChange} />
      ) : null}

      {draft.auth_mode === "none" ? (
        <Alert data-testid="settings-providers-editor-auth-none-note" variant="info">
          <AlertDescription className="text-form-hint">
            CompozyOS doesn&apos;t pass any keys to this provider.
          </AlertDescription>
        </Alert>
      ) : null}
    </FormSection>
  );
}

interface ProviderNativeAuthFieldsProps {
  draft: ProviderDraft;
  entry: SettingsProviderEntry | null;
  onChange: ProviderDraftChange;
}

/**
 * Provider-owned login exposes one editable status command and one safe,
 * read-only descriptor for the write-only login command.
 */
function ProviderNativeAuthFields({ draft, entry, onChange }: ProviderNativeAuthFieldsProps) {
  const authStatus = entry?.auth_status ?? null;

  return (
    <>
      {authStatus?.state ? (
        <Alert data-testid="settings-providers-editor-auth-status" variant="info">
          <AlertDescription className="flex flex-col gap-1 text-form-hint">
            <span>Sign-in status: {providerAuthStateLabel(authStatus.state)}</span>
            {authStatus.message ? <span className="text-muted">{authStatus.message}</span> : null}
          </AlertDescription>
        </Alert>
      ) : null}

      <ModalSettingsFieldRow
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-providers-editor-auth-status-command-input"
            onChange={event =>
              onChange(current => ({ ...current, auth_status_command: event.target.value }))
            }
            placeholder="codex auth status"
            value={draft.auth_status_command}
          />
        }
        data-testid="settings-providers-editor-auth-status-command"
        description="Command CompozyOS runs to check whether you're signed in."
        label={
          <>
            Status command
            <Eyebrow className="ml-1.5 text-subtle">optional</Eyebrow>
          </>
        }
      />

      <ModalSettingsFieldRow
        control={
          <ProviderLoginDescriptorView
            login={authStatus?.login ?? null}
            testId="settings-providers-editor-auth-login-descriptor"
          />
        }
        data-testid="settings-providers-editor-auth-login"
        description="Only the app name and whether it's installed are shown here."
        label="Sign-in app"
      />
    </>
  );
}
