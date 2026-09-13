import { ChevronDown, FolderOpen, Plus } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { GithubLogo } from "@compozy/ui/logos";

import type { ExtensionInstallSource } from "./extension-install-model";

interface MarketplaceAddMenuProps {
  onInstall: (source: ExtensionInstallSource) => void;
}

/**
 * The one head door for extensions the catalog does not carry: install from a GitHub release or
 * from a local build, both through the production install dialog.
 */
function MarketplaceAddMenu({ onInstall }: MarketplaceAddMenuProps) {
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
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export { MarketplaceAddMenu };
export type { MarketplaceAddMenuProps };
