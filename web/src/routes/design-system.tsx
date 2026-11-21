import { createFileRoute } from "@tanstack/react-router";

import { DesignSystemShowcase } from "@/components/design-system-showcase";
import { useRouterScrollTracking } from "@/lib/router-scroll-restoration";

export const Route = createFileRoute("/design-system")({
  component: DesignSystemPage,
});

function DesignSystemPage() {
  // The page outside the shell restores scroll by the router (`restoreScrollOutsideShell`).
  useRouterScrollTracking();
  return <DesignSystemShowcase />;
}
