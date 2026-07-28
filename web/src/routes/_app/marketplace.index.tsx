import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/_app/marketplace/")({
  beforeLoad: redirectToMarketplaceSkills,
  component: MarketplaceIndexRedirect,
});

function redirectToMarketplaceSkills() {
  throw redirect({ to: "/marketplace/skills" });
}

function MarketplaceIndexRedirect() {
  return null;
}
