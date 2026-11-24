import { useNavigate } from "@tanstack/react-router";
import { CircleAlert } from "@compozy/ui/icons";

import { Alert, AlertActions, AlertTitle, Button } from "@compozy/ui";

import type { ExtensionUnmetDependency } from "@/systems/extensions";

const ACTION_LABEL: Record<string, string> = {
  disabled: "Turn on",
  version_unmet: "Update",
};

/**
 * A dependency this instance no longer resolves (removed or turned off with force, or too old).
 * The action routes to that extension: browse to install it, or Installed to turn it on or update.
 */
export function MarketplaceExtensionDependencyNotice({
  dependencies,
}: {
  dependencies: readonly ExtensionUnmetDependency[];
}) {
  const navigate = useNavigate();
  const openDependency = (dependency: ExtensionUnmetDependency) =>
    void navigate({
      search: { q: dependency.name },
      to: dependency.state === "missing" ? "/marketplace" : "/marketplace/installed",
    });
  return dependencies.map(dependency => (
    <Alert
      data-testid={`marketplace-extension-dependency-notice-${dependency.name}`}
      key={dependency.name}
      role="status"
      variant="warning"
    >
      <CircleAlert aria-hidden="true" />
      <AlertTitle>Missing dependency: {dependency.name}</AlertTitle>
      <AlertActions>
        <Button
          onClick={() => openDependency(dependency)}
          size="sm"
          type="button"
          variant="neutral"
        >
          {ACTION_LABEL[dependency.state] ?? "Install"}
        </Button>
      </AlertActions>
    </Alert>
  ));
}
