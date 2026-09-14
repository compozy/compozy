import { useState } from "react";
import { toast } from "sonner";

import {
  marketplaceErrorMessage,
  useRefreshMarketplaceSource,
  useRemoveMarketplaceSource,
  useUpdateMarketplaceSource,
} from "@/systems/marketplace";

/**
 * Row actions over the canonical source mutations: a switch patches `enabled`, Try again / Refresh
 * now reads the document again, Remove asks first. Each failure lands as a toast naming the
 * daemon's message; the rows keep rendering the last authoritative list.
 */
export function useMarketplaceSourceActions() {
  const update = useUpdateMarketplaceSource();
  const refresh = useRefreshMarketplaceSource();
  const remove = useRemoveMarketplaceSource();
  const [removing, setRemoving] = useState<string | null>(null);

  const pendingNames = new Set<string>();
  if (update.isPending && update.variables) pendingNames.add(update.variables.name);
  if (refresh.isPending && refresh.variables) pendingNames.add(refresh.variables);
  if (remove.isPending && remove.variables) pendingNames.add(remove.variables);

  return {
    isPending: (name: string) => pendingNames.has(name),
    toggle: (name: string, enabled: boolean) => {
      update.mutate(
        { name, body: { enabled } },
        {
          onError: error => {
            toast.error(
              marketplaceErrorMessage(
                error,
                enabled ? `Could not turn on ${name}` : `Could not turn off ${name}`
              )
            );
          },
        }
      );
    },
    refresh: (name: string) => {
      refresh.mutate(name, {
        onSuccess: ({ source }) => {
          if (source.state === "degraded") {
            toast.warning(`Could not refresh ${name}`, {
              description: source.error ?? source.error_class ?? undefined,
            });
          }
        },
        onError: error => {
          toast.error(marketplaceErrorMessage(error, `Could not refresh ${name}`));
        },
      });
    },
    removing,
    askRemove: (name: string) => {
      remove.reset();
      setRemoving(name);
    },
    cancelRemove: () => {
      remove.reset();
      setRemoving(null);
    },
    confirmRemove: () => {
      if (removing === null) return;
      const name = removing;
      remove.mutate(name, {
        onSuccess: () => {
          setRemoving(null);
          toast.success(`${name} removed`, {
            description: "Extensions you installed from it stay installed.",
          });
        },
      });
    },
    removeError:
      remove.error && removing !== null
        ? marketplaceErrorMessage(remove.error, `Could not remove ${removing}`)
        : null,
    isRemoving: remove.isPending,
  };
}

export type MarketplaceSourceActions = ReturnType<typeof useMarketplaceSourceActions>;
