import { KeyRound, Trash2 } from "lucide-react";

import {
  Button,
  CatalogCard,
  CatalogCardSkeleton,
  Pill,
  Skeleton,
  SkeletonText,
  Time,
} from "@compozy/ui";

import { vaultSecretLocation, vaultSecretTitle } from "../lib/vault-secret-title";
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
  const title = vaultSecretTitle(secret.ref);

  return (
    <CatalogCard
      actionable={selectable}
      data-ref={secret.ref}
      data-testid="vault-secrets-card"
      selected={selected}
    >
      <button
        aria-label={`Open ${title}`}
        className="flex min-w-0 flex-col gap-3 text-left"
        data-testid={`vault-secrets-select-${secret.ref}`}
        disabled={!selectable}
        onClick={() => onSelect?.(secret)}
        title={secret.ref}
        type="button"
      >
        <div className="flex items-start gap-2.5">
          <CatalogCard.Logo>
            <KeyRound aria-hidden="true" className="size-3.5" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title>{title}</CatalogCard.Title>
            <span className="truncate text-eyebrow text-subtle">
              {vaultSecretLocation(secret.ref) || secret.namespace}
            </span>
          </div>
        </div>
      </button>
      <CatalogCard.Actions className="justify-between">
        {trimmedKind ? (
          <Pill mono data-testid={`vault-secrets-kind-${secret.ref}`} size="sm" tone="neutral">
            {trimmedKind}
          </Pill>
        ) : null}
        <div className="ml-auto flex items-center gap-1">
          <Time
            className="text-eyebrow text-faint"
            data-testid={`vault-secrets-updated-${secret.ref}`}
            iso={secret.updated_at}
          />
          {onDelete ? (
            <Button
              aria-label={`Delete ${title}`}
              data-testid={`vault-secrets-delete-${secret.ref}`}
              onClick={event => {
                event.stopPropagation();
                onDelete(secret);
              }}
              size="icon-sm"
              type="button"
              variant="quiet"
            >
              <Trash2 aria-hidden="true" />
            </Button>
          ) : null}
        </div>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

const SECRET_CARD_SKELETONS = [
  { id: "secret-card-skeleton-1", title: "w-40", location: "w-32" },
  { id: "secret-card-skeleton-2", title: "w-32", location: "w-40" },
  { id: "secret-card-skeleton-3", title: "w-48", location: "w-28" },
  { id: "secret-card-skeleton-4", title: "w-36", location: "w-36" },
  { id: "secret-card-skeleton-5", title: "w-44", location: "w-32" },
  { id: "secret-card-skeleton-6", title: "w-28", location: "w-40" },
] as const;

/**
 * `count` placeholder cards with the secret card's own header (glyph, name and one location line)
 * and its foot: the kind pill opposite the updated time and the delete control.
 */
export function VaultSecretsCardSkeleton({ count = 6 }: { count?: number }) {
  return (
    <>
      {SECRET_CARD_SKELETONS.slice(0, count).map(card => (
        <CatalogCardSkeleton key={card.id}>
          <div className="flex items-start gap-2.5">
            <Skeleton className="size-(--size-catalog-logo) shrink-0 rounded-sm" />
            <div className="flex min-w-0 flex-1 flex-col gap-1">
              <CatalogCard.Title>
                <SkeletonText className={card.title} />
              </CatalogCard.Title>
              <span className="block truncate text-eyebrow">
                <SkeletonText className={card.location} />
              </span>
            </div>
          </div>
          <CatalogCard.Actions className="justify-between">
            <Skeleton className="h-pill-sm w-16 rounded-pill" />
            <Skeleton className="ml-auto size-button-icon-sm" />
          </CatalogCard.Actions>
        </CatalogCardSkeleton>
      ))}
    </>
  );
}
