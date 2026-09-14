import { ChevronDown, FolderOpen, Plus, Store } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { GithubLogo } from "@compozy/ui/logos";

import type { ExtensionInstallSource } from "./extension-install-model";

interface MarketplaceAddMenuProps {
  onInstall: (source: ExtensionInstallSource) => void;
  /** Opens the shared plugin-marketplace dialog (the third door, after the two installers). */
  onAddMarketplace: () => void;
}

/**
 * The one head door for things the catalog does not carry yet: install an extension from a GitHub
 * release or a local build through the production install dialog, or register a plugin
 * marketplace so its plugins list under their own section.
 */
function MarketplaceAddMenu({ onInstall, onAddMarketplace }: MarketplaceAddMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label="Add"
            data-testid="marketplace-add"
            size="sm"
            type="button"
            variant="outline"
          />
        }
      >
        <Plus aria-hidden="true" className="size-3" />
        Add
        <ChevronDown aria-hidden="true" className="size-3 text-subtle" data-icon="inline-end" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-62" data-testid="marketplace-add-menu">
        <DropdownMenuItem data-testid="marketplace-add-github" onClick={() => onInstall("github")}>
          <GithubLogo aria-hidden="true" className="size-4 text-muted" />
          Install extension from GitHub…
          <span className="ml-auto font-mono text-mono-id text-faint">owner/repo</span>
        </DropdownMenuItem>
        <DropdownMenuItem
          data-testid="marketplace-add-local"
          onClick={() => onInstall("local_path")}
        >
          <FolderOpen aria-hidden="true" className="size-4 text-muted" />
          Install extension from a local build…
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem data-testid="marketplace-add-marketplace" onClick={onAddMarketplace}>
          <Store aria-hidden="true" className="size-4 text-muted" />
          Add plugin marketplace…
          <span className="ml-auto font-mono text-mono-id text-faint">marketplace.json</span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export { MarketplaceAddMenu };
export type { MarketplaceAddMenuProps };
