import { KeyRound } from "lucide-react";

import {
  DataSurface,
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
  Time,
} from "@agh/ui";

import type { VaultSecret } from "../types";

interface SessionVaultPanelProps {
  secrets: readonly VaultSecret[];
  isLoading?: boolean;
  error?: Error | null;
  sessionId?: string;
}

function displayVaultRef(secret: VaultSecret, sessionId?: string): string {
  const prefix = sessionId ? `vault:sessions/${sessionId}/` : "";
  if (prefix && secret.ref.startsWith(prefix)) {
    return secret.ref.slice(prefix.length);
  }
  return secret.ref;
}

export function SessionVaultPanel({
  secrets,
  isLoading = false,
  error = null,
  sessionId,
}: SessionVaultPanelProps) {
  return (
    <DataSurface
      state={isLoading ? "loading" : error ? "error" : secrets.length === 0 ? "empty" : "ready"}
    >
      <DataSurface.Loading
        data-testid="session-inspector-vault-loading"
        label="Loading session vault metadata"
        size="sm"
        surface="bare"
      />
      <DataSurface.Error
        icon={KeyRound}
        title="Vault unavailable"
        description={error?.message}
        data-testid="session-inspector-vault-error"
      />
      <DataSurface.Empty
        icon={KeyRound}
        title="No session vault secrets"
        description="Session-scoped vault metadata appears here when tools store write-only values."
        data-testid="session-inspector-vault-empty"
      />
      <DataSurface.Content>
        <ItemGroup data-testid="session-inspector-vault-list">
          {secrets.map(secret => (
            <Item key={secret.ref} data-testid="session-inspector-vault-row">
              <ItemMedia>
                <span
                  aria-hidden="true"
                  className="inline-flex size-6 items-center justify-center rounded-sm bg-canvas-soft text-muted"
                >
                  <KeyRound className="size-3" />
                </span>
              </ItemMedia>
              <ItemContent>
                <ItemTitle
                  className="text-small-body text-fg-strong"
                  data-testid="session-inspector-vault-title"
                >
                  Vault secret
                </ItemTitle>
                <ItemDescription
                  className="block min-w-0 truncate font-mono text-eyebrow text-faint"
                  data-testid="session-inspector-vault-ref"
                  title={secret.ref}
                >
                  {displayVaultRef(secret, sessionId)}
                </ItemDescription>
              </ItemContent>
              <ItemActions>
                <Time
                  className="shrink-0 font-mono text-badge text-subtle"
                  data-testid="session-inspector-vault-updated"
                  iso={secret.updated_at}
                />
              </ItemActions>
            </Item>
          ))}
        </ItemGroup>
      </DataSurface.Content>
    </DataSurface>
  );
}
