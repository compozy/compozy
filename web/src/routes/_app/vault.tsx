import { createFileRoute } from "@tanstack/react-router";

import { validateVaultSearch } from "@/systems/vault";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadVaultRoute } from "./-vault-preload";

export const Route = createFileRoute("/_app/vault")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Vault", to: "/vault" } },
  }),
  validateSearch: validateVaultSearch,
  loaderDeps: ({ search }) => ({ namespace: search.namespace, prefix: search.q }),
  loader: ({ context, deps }) => preloadVaultRoute(context.queryClient, deps),
  component: createOsRouteSync("vault"),
});
