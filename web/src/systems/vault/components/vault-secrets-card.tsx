import { KeyRound, Trash2 } from "lucide-react";

import { Button, CatalogCard, Pill, Time } from "@agh/ui";

import { vaultNamespaceTone } from "../lib/vault-tones";
import type { VaultSecret } from "../types";

export interface VaultSecretsCardProps {
  secret: VaultSecret;
  selected?: boolean;
  onSelect?: (secret: VaultSecret) => void;
  onDelete?: (secret: VaultSecret) => void;
}

export function VaultSecretsCard({
  secret,
  selected = false,
  onSelect,
  onDelete,
}: VaultSecretsCardProps) {
  const trimmedKind = secret.kind?.trim();
  const selectable = onSelect !== undefined;

  return (
    <CatalogCard
      actionable={selectable}
      data-ref={secret.ref}
      data-testid="vault-secrets-card"
      selected={selected}
    >
      <button
        aria-label={`Inspect ${secret.ref}`}
        className="flex min-w-0 flex-col gap-3 text-left"
        data-testid={`vault-secrets-select-${secret.ref}`}
        disabled={!selectable}
        onClick={() => onSelect?.(secret)}
        type="button"
      >
        <div className="flex items-start gap-2.5">
          <CatalogCard.Logo>
            <KeyRound aria-hidden="true" className="size-3.5" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title className="font-mono text-xs font-medium">
              {secret.ref}
            </CatalogCard.Title>
            <Pill mono size="sm" tone={vaultNamespaceTone(secret.namespace)}>
              {secret.namespace}
            </Pill>
          </div>
        </div>
      </button>
      <CatalogCard.Actions className="justify-between">
        {trimmedKind ? (
          <Pill mono data-testid={`vault-secrets-kind-${secret.ref}`} size="sm" tone="neutral">
            {trimmedKind}
          </Pill>
        ) : (
          <span
            className="font-mono text-mono-id text-faint"
            data-testid={`vault-secrets-kind-empty-${secret.ref}`}
          >
            --
          </span>
        )}
        <div className="flex items-center gap-1">
          <Time
            className="font-mono text-mono-id text-faint"
            data-testid={`vault-secrets-updated-${secret.ref}`}
            iso={secret.updated_at}
          />
          {onDelete ? (
            <Button
              aria-label={`Delete ${secret.ref}`}
              data-testid={`vault-secrets-delete-${secret.ref}`}
              onClick={event => {
                event.stopPropagation();
                onDelete(secret);
              }}
              size="icon-sm"
              type="button"
              variant="ghost"
            >
              <Trash2 aria-hidden="true" className="size-3" />
            </Button>
          ) : null}
        </div>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
