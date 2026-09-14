import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useState, type ChangeEvent } from "react";

import { useAddMarketplaceSource } from "../hooks/use-marketplace-sources";
import { marketplaceSourcePreviewOptions } from "../lib/query-options";
import type { MarketplaceSource, MarketplaceSourcePreview } from "../types";
import {
  describeMarketplaceFailure,
  normalizeMarketplaceDraft,
  sameMarketplaceDraft,
  type AddMarketplaceDraft,
  type AddMarketplaceFailure,
} from "./add-marketplace-model";

const EMPTY_DRAFT: AddMarketplaceDraft = { ref: "", name: "" };

export interface AddMarketplaceFormOptions {
  onOpenChange: (open: boolean) => void;
  onAdded?: (source: MarketplaceSource) => void;
}

/**
 * The dialog's behavior: leaving a field asks the daemon to read the document without registering
 * (`?dry_run=true`) through the shared preview query; editing either field discards that check and
 * cancels any in-flight read, so Add only ever registers the exact reference and name the last
 * successful check described.
 */
export function useAddMarketplaceForm({ onOpenChange, onAdded }: AddMarketplaceFormOptions) {
  const [draft, setDraft] = useState<AddMarketplaceDraft>(EMPTY_DRAFT);
  const [checked, setChecked] = useState<AddMarketplaceDraft | null>(null);
  const [nameVisible, setNameVisible] = useState(false);
  const add = useAddMarketplaceSource();
  const preview = useQuery(
    marketplaceSourcePreviewOptions(
      { ref: checked?.ref ?? "", ...(checked?.name ? { name: checked.name } : {}) },
      checked !== null
    )
  );
  const resetAdd = add.reset;

  const { checking, found, failure, canAdd } = marketplaceFormReadiness(
    draft,
    checked,
    preview,
    add.error,
    add.isPending
  );

  const edit = (patch: Partial<AddMarketplaceDraft>) => {
    setDraft(current => ({ ...current, ...patch }));
    setChecked(null);
    resetAdd();
  };

  const check = (next: AddMarketplaceDraft = draft) => {
    const normalized = normalizeMarketplaceDraft(next);
    if (normalized.ref === "") return;
    if (checked !== null && sameMarketplaceDraft(checked, normalized)) return;
    resetAdd();
    setChecked(normalized);
  };

  const close = (next: boolean) => {
    if (!next) {
      setDraft(EMPTY_DRAFT);
      setChecked(null);
      setNameVisible(false);
      resetAdd();
    }
    onOpenChange(next);
  };

  return {
    draft,
    checking,
    found,
    failure,
    canAdd,
    isPending: add.isPending,
    showNameField: nameVisible || failure?.field === "name",
    refInvalid: failure?.field === "ref",
    nameInvalid: failure?.field === "name",
    editRef: (event: ChangeEvent<HTMLInputElement>) => edit({ ref: event.target.value }),
    editName: (event: ChangeEvent<HTMLInputElement>) => edit({ name: event.target.value }),
    check: () => check(),
    useSuggestedName: (name: string) => {
      const next = { ...draft, name };
      setDraft(next);
      setNameVisible(true);
      resetAdd();
      setChecked(normalizeMarketplaceDraft(next));
    },
    close,
    cancel: () => close(false),
    submit: () => {
      if (add.isPending) return;
      if (!canAdd || checked === null) {
        check();
        return;
      }
      add.mutate(
        { ref: checked.ref, ...(checked.name ? { name: checked.name } : {}) },
        {
          onSuccess: response => {
            close(false);
            onAdded?.(response.source);
          },
        }
      );
    },
  };
}

export type AddMarketplaceForm = ReturnType<typeof useAddMarketplaceForm>;

function marketplaceFormReadiness(
  draft: AddMarketplaceDraft,
  checked: AddMarketplaceDraft | null,
  preview: UseQueryResult<MarketplaceSourcePreview>,
  addError: Error | null,
  adding: boolean
) {
  const checking = checked !== null && preview.isFetching;
  const found: MarketplaceSourcePreview | null =
    checked !== null && preview.isSuccess && !preview.isFetching ? preview.data : null;
  const failureSource = addError ?? (checked !== null && !checking ? preview.error : null);
  const failure: AddMarketplaceFailure | null = failureSource
    ? describeMarketplaceFailure(
        failureSource,
        checked ?? draft,
        addError ? "The marketplace could not be added." : "The marketplace could not be read."
      )
    : null;
  const canAdd = found !== null && checked !== null && !adding;
  return { checking, found, failure, canAdd };
}
