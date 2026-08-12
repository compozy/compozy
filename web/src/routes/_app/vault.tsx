import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { validateVaultSearch } from "@/systems/vault";

export const Route = createFileRoute("/_app/vault")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Vault", to: "/vault" } },
  }),
  validateSearch: validateVaultSearch,
  loaderDeps: ({ search }) => ({ namespace: search.namespace, prefix: search.q }),
  loader: async ({ context, deps }) =>
    (await import("./-vault-preload")).preloadVaultRoute(context.queryClient, deps),
  component: createOsRouteSync("vault"),
});
