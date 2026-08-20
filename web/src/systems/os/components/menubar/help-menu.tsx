import { ArrowUpRight } from "lucide-react";

import { MenubarContent, MenubarItem, MenubarMenu, MenubarTrigger } from "@compozy/ui";

import { usePaletteRegistry } from "../../hooks/use-palette-registry";
import { MenubarCommandGroups } from "./menubar-command-groups";
import { MenubarCommandItem } from "./menubar-command-item";

export interface HelpMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Runs a registry command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

const EXTERNAL_LINKS = [
  { id: "documentation", label: "Documentation", href: "https://compozy.com/docs/" },
  { id: "protocol", label: "Protocol", href: "https://compozy.com/docs/network/protocol/" },
  { id: "marketplace", label: "Marketplace", href: "https://compozy.com/marketplace/" },
  { id: "changelog", label: "What's new", href: "https://compozy.com/changelog/" },
] as const;

const ISSUES_URL = "https://github.com/compozy/compozy/issues";

/**
 * Help menu. External destinations carry an arrow glyph so "this leaves the
 * app" is shape, not color; support routes to Settings → Observability, where
 * the two-step support-bundle consent already lives.
 */
export function HelpMenu({ open, onOpenChange, onRun }: HelpMenuProps) {
  const registry = usePaletteRegistry();
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Help</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-help">
        <MenubarCommandGroups
          groups={[
            {
              id: "shortcuts",
              content: registry.byId.has("shortcuts.cheatsheet") ? (
                <MenubarCommandItem commandId="shortcuts.cheatsheet" onRun={onRun} />
              ) : null,
            },
            {
              id: "links",
              content: EXTERNAL_LINKS.map(link => (
                <ExternalMenuItem
                  key={link.id}
                  testId={`os-menu-${link.id}`}
                  href={link.href}
                  label={link.label}
                />
              )),
            },
            {
              id: "support",
              content: (
                <>
                  <ExternalMenuItem
                    testId="os-menu-report-issue"
                    href={ISSUES_URL}
                    label="Report an issue"
                  />
                  <MenubarCommandItem commandId="settings.observability" onRun={onRun} />
                </>
              ),
            },
          ]}
        />
      </MenubarContent>
    </MenubarMenu>
  );
}

function ExternalMenuItem({
  testId,
  href,
  label,
}: {
  testId: string;
  href: string;
  label: string;
}) {
  return (
    <MenubarItem
      data-testid={testId}
      render={
        <a
          href={href}
          target="_blank"
          rel="noreferrer"
          aria-label={`${label} (opens in a new tab)`}
        />
      }
    >
      {label}
      <ArrowUpRight className="ml-auto size-3.5 text-faint" />
    </MenubarItem>
  );
}
