import { useState } from "react";
import { Check, Copy, KeyRound, Plus } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  dialogShellClass,
  Empty,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Field,
  FieldDescription,
  FieldLabel,
  Input,
  KindIcon,
  ListingRow,
  MonoId,
  Switch,
  toast,
} from "@compozy/ui";

import type { AccountTokenFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import type { CreatedAccountTokenFixture } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
import { SettingRow } from "@/systems/settings/components/setting-row";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

export interface AccountEmailSectionProps {
  marketingOptIn: boolean;
  onToggleMarketingOptIn: (next: boolean) => void;
}

/**
 * Settings → Account → Email (`_uiux.md` U3, US-028 AC-1):
 * single marketing preference switch; transactional email is never listed.
 */
export function AccountEmailSection({
  marketingOptIn,
  onToggleMarketingOptIn,
}: AccountEmailSectionProps) {
  return (
    <SettingsGroup data-testid="account-email" title="Email">
      <SettingRow
        control={
          <Switch
            aria-label="Product news and offers"
            checked={marketingOptIn}
            onCheckedChange={onToggleMarketingOptIn}
            size="sm"
          />
        }
        description="What's new in Compozy, tips and occasional offers."
        label="Product news and offers"
      />
    </SettingsGroup>
  );
}

export interface AccountTokensSectionProps {
  tokens: readonly AccountTokenFixture[];
  onCreateToken: () => void;
  onRevokeToken: (token: AccountTokenFixture) => void;
}

/**
 * Settings → Account → Account tokens (`_uiux.md` U3, US-029):
 * local-only tokens for CI and ephemeral environments.
 */
export function AccountTokensSection({
  tokens,
  onCreateToken,
  onRevokeToken,
}: AccountTokensSectionProps) {
  return (
    <SettingsGroup
      action={
        <Button onClick={onCreateToken} size="sm" type="button" variant="secondary">
          <Plus aria-hidden="true" />
          Create token
        </Button>
      }
      data-testid="account-tokens"
      description="Sign in CI and throwaway environments with COMPOZY_ACCOUNT_TOKEN. Unlocks local use only."
      title={tokens.length > 0 ? `Account tokens (${tokens.length})` : "Account tokens"}
    >
      {tokens.length === 0 ? (
        <Empty
          action={
            <Button onClick={onCreateToken} size="sm" type="button" variant="secondary">
              <Plus aria-hidden="true" />
              Create token
            </Button>
          }
          className="px-6 py-8"
          description="Create a token for CI jobs or containers without a browser."
          fill={false}
          icon={KeyRound}
          size="compact"
          title="No account tokens"
        />
      ) : (
        tokens.map(token => (
          <ListingRow
            className="min-h-14 py-3"
            data-token-id={token.id}
            interactive={false}
            key={token.id}
          >
            <ListingRow.Icon>
              <KindIcon aria-hidden="true" icon={KeyRound} size="sm" tone="muted" />
            </ListingRow.Icon>

            <ListingRow.Main>
              <ListingRow.Name>
                <ListingRow.Title>{token.name}</ListingRow.Title>
              </ListingRow.Name>
              <ListingRow.Meta className="mt-1">
                <span>Created {token.createdLabel}</span>
                <ListingRow.MetaDot />
                <span>
                  {token.lastUsedLabel ? `Last used ${token.lastUsedLabel}` : "Never used"}
                </span>
                <ListingRow.MetaDot />
                <MonoId value={token.id} />
              </ListingRow.Meta>
            </ListingRow.Main>

            <ListingRow.Trail>
              <Button
                onClick={() => onRevokeToken(token)}
                size="sm"
                type="button"
                variant="outline"
              >
                Revoke
              </Button>
            </ListingRow.Trail>
          </ListingRow>
        ))
      )}
    </SettingsGroup>
  );
}

export interface AccountCreateTokenDialogProps {
  open: boolean;
  onClose: () => void;
  onCreate: (name: string) => void;
}

/** Prompts for a short name before generating an account token. */
export function AccountCreateTokenDialog({
  open,
  onClose,
  onCreate,
}: AccountCreateTokenDialogProps) {
  const [name, setName] = useState("");

  if (!open) return null;

  const trimmed = name.trim();

  const handleSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    if (!trimmed) return;
    onCreate(trimmed);
    setName("");
  };

  return (
    <Dialog
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open={open}
    >
      <DialogContent
        className={dialogShellClass("sm")}
        data-testid="account-create-token-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Use account tokens in CI or ephemeral environments without a browser."
          icon={KeyRound}
          onClose={onClose}
          title="Create account token"
        />
        <form onSubmit={handleSubmit}>
          <EntityDialogBody>
            <Field>
              <FieldLabel htmlFor="account-token-name">Token name</FieldLabel>
              <Input
                autoComplete="off"
                id="account-token-name"
                onChange={event => setName(event.target.value)}
                placeholder="ci"
                value={name}
              />
              <FieldDescription>
                Helps you identify which environment uses this token if you need to revoke it later.
              </FieldDescription>
            </Field>
          </EntityDialogBody>
          <EntityDialogFooter
            cancelLabel="Cancel"
            hint="Unlocks local use only."
            onCancel={onClose}
            onPrimary={() => {
              if (!trimmed) return;
              onCreate(trimmed);
              setName("");
            }}
            primaryDisabled={!trimmed}
            primaryLabel="Create token"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}

export interface AccountTokenCreatedDialogProps {
  createdToken: CreatedAccountTokenFixture | null;
  onClose: () => void;
}

/**
 * One-time reveal dialog after creating an account token (`_uiux.md` U3, US-029 AC-1):
 * shows the secret once with Copy and exact local-only guidance.
 */
export function AccountTokenCreatedDialog({
  createdToken,
  onClose,
}: AccountTokenCreatedDialogProps) {
  const [copied, setCopied] = useState(false);

  if (!createdToken) return null;

  const handleCopy = () => {
    void navigator.clipboard?.writeText(createdToken.token).catch(() => undefined);
    setCopied(true);
    toast.success("Account token copied.");
  };

  return (
    <Dialog
      onOpenChange={next => {
        if (!next) onClose();
      }}
      open
    >
      <DialogContent
        className={dialogShellClass("sm")}
        data-testid="account-token-created-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`Token "${createdToken.name}" is ready. Copy it now; it won't be shown again.`}
          icon={KeyRound}
          onClose={onClose}
          title="Account token created"
        />

        <EntityDialogBody className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-2 rounded-md border border-line bg-code-bg px-3 py-2.5">
            <code className="min-w-0 flex-1 font-mono text-small-body break-all text-fg-strong select-all">
              {createdToken.token}
            </code>
            <Button
              className="shrink-0"
              onClick={handleCopy}
              size="sm"
              type="button"
              variant="secondary"
            >
              {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>

          <Alert variant="info">
            <AlertDescription>
              Use it with <code className="font-mono text-fg">COMPOZY_ACCOUNT_TOKEN</code>. It
              unlocks local use only.
            </AlertDescription>
          </Alert>
        </EntityDialogBody>

        <DialogFooter className="min-h-editor-footer items-center" variant="ruled">
          <Button onClick={onClose} type="button">
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
