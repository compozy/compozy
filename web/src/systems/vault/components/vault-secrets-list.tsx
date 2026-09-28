import { KeyRound } from "lucide-react";
import type { ReactNode } from "react";

import { DataSurface, type ListingViewMode } from "@compozy/ui";

import type { VaultSecret } from "../types";
import { VaultSecretsCard } from "./vault-secrets-card";
import { VaultSecretsRow } from "./vault-secrets-row";

interface VaultSecretsListProps {
  secrets: VaultSecret[];
  view?: ListingViewMode;
  selectedRef?: string | null;
  isLoading?: boolean;
  error?: Error | null;
  onSelect?: (secret: VaultSecret) => void;
  onDelete?: (secret: VaultSecret) => void;
  emptyTitle?: string;
  emptyDescription?: string;
  emptyAction?: ReactNode;
  "data-testid"?: string;
}

export function VaultSecretsList({
  secrets,
  view = "rows",
  selectedRef = null,
  isLoading = false,
  error = null,
  onSelect,
  onDelete,
  emptyTitle = "No secrets yet",
  emptyDescription = "Secrets you save show up here.",
  emptyAction,
  "data-testid": testId = "vault-secrets-list",
}: VaultSecretsListProps) {
  return (
    <DataSurface
      className="flex min-h-0 flex-1 flex-col"
      state={isLoading ? "loading" : error ? "error" : secrets.length === 0 ? "empty" : "ready"}
    >
      <DataSurface.Loading data-testid={`${testId}-loading`} label="Loading secrets" />
      <DataSurface.Error
        description={error?.message}
        icon={KeyRound}
        title="Couldn't load your secrets"
        data-testid={`${testId}-error`}
      />
      <DataSurface.Empty
        action={emptyAction}
        description={emptyDescription}
        icon={KeyRound}
        title={emptyTitle}
        data-testid={`${testId}-empty`}
      />
      <DataSurface.Content className="min-w-0" data-testid={testId}>
        {view === "cards" ? (
          <div
            className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 xl:grid-cols-3"
            data-testid={`${testId}-card-grid`}
          >
            {secrets.map(secret => (
              <VaultSecretsCard
                key={secret.ref}
                onDelete={onDelete}
                onSelect={onSelect}
                secret={secret}
                selected={selectedRef === secret.ref}
              />
            ))}
          </div>
        ) : (
          <div
            className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
            data-testid={`${testId}-rows`}
          >
            {secrets.map(secret => (
              <VaultSecretsRow
                key={secret.ref}
                onDelete={onDelete}
                onSelect={onSelect}
                secret={secret}
                selected={selectedRef === secret.ref}
              />
            ))}
          </div>
        )}
      </DataSurface.Content>
    </DataSurface>
  );
}
