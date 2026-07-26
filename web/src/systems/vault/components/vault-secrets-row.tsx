import { KeyRound, Trash2 } from "lucide-react";

import { Button, ListingRow, Pill, Time } from "@agh/ui";

import { vaultNamespaceTone } from "../lib/vault-tones";
import type { VaultSecret } from "../types";

export interface VaultSecretsRowProps {
  secret: VaultSecret;
  selected?: boolean;
  onSelect?: (secret: VaultSecret) => void;
  onDelete?: (secret: VaultSecret) => void;
}

export function VaultSecretsRow({
  secret,
  selected = false,
  onSelect,
  onDelete,
}: VaultSecretsRowProps) {
  const trimmedKind = secret.kind?.trim();
  const selectable = onSelect !== undefined;

  return (
    <ListingRow
      data-testid="vault-secrets-row"
      data-ref={secret.ref}
      interactive={selectable}
      selected={selected}
    >
      {selectable ? (
        <ListingRow.Link
          render={
            <button
              aria-label={`Inspect ${secret.ref}`}
              data-testid={`vault-secrets-select-${secret.ref}`}
              onClick={() => onSelect(secret)}
              type="button"
            />
          }
        >
          <VaultSecretsRowBody secret={secret} />
        </ListingRow.Link>
      ) : (
        <VaultSecretsRowBody secret={secret} />
      )}
      <ListingRow.Trail>
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
      </ListingRow.Trail>
    </ListingRow>
  );
}

function VaultSecretsRowBody({ secret }: { secret: VaultSecret }) {
  return (
    <>
      <ListingRow.Icon>
        <KeyRound aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name mono>
          <ListingRow.Title>{secret.ref}</ListingRow.Title>
          <Pill mono size="sm" tone={vaultNamespaceTone(secret.namespace)}>
            {secret.namespace}
          </Pill>
        </ListingRow.Name>
        <ListingRow.Meta>
          <span>updated</span>
          <Time
            className="font-mono text-mono-id text-faint"
            data-testid={`vault-secrets-updated-${secret.ref}`}
            iso={secret.updated_at}
          />
        </ListingRow.Meta>
      </ListingRow.Main>
    </>
  );
}
