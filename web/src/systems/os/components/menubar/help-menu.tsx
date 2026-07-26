import { ArrowUpRight } from "lucide-react";

import {
  MenubarContent,
  MenubarItem,
  MenubarMenu,
  MenubarSeparator,
  MenubarTrigger,
} from "@agh/ui";

export interface HelpMenuProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Opening the observability settings surface needs a live client. */
  canOpenApps: boolean;
  onOpenShortcuts: () => void;
  onOpenSupport: () => void;
}

const EXTERNAL_LINKS = [
  { id: "documentation", label: "Documentation", href: "https://agh.network/runtime/" },
  { id: "protocol", label: "Protocol", href: "https://agh.network/protocol/" },
  { id: "changelog", label: "What's new", href: "https://agh.network/changelog/" },
] as const;

const ISSUES_URL = "https://github.com/compozy/agh/issues";

/**
 * Help menu. External destinations carry an arrow glyph so "this leaves the
 * app" is shape, not color; support routes to Settings → Observability, where
 * the two-step support-bundle consent already lives.
 */
export function HelpMenu({
  open,
  onOpenChange,
  canOpenApps,
  onOpenShortcuts,
  onOpenSupport,
}: HelpMenuProps) {
  return (
    <MenubarMenu open={open} onOpenChange={onOpenChange}>
      <MenubarTrigger>Help</MenubarTrigger>
      <MenubarContent align="start" data-testid="os-menu-help">
        <MenubarItem data-testid="os-menu-shortcuts" onClick={onOpenShortcuts}>
          Keyboard shortcuts…
        </MenubarItem>
        <MenubarSeparator />
        {EXTERNAL_LINKS.map(link => (
          <ExternalMenuItem
            key={link.id}
            testId={`os-menu-${link.id}`}
            href={link.href}
            label={link.label}
          />
        ))}
        <MenubarSeparator />
        <ExternalMenuItem testId="os-menu-report-issue" href={ISSUES_URL} label="Report an issue" />
        <MenubarItem data-testid="os-menu-support" disabled={!canOpenApps} onClick={onOpenSupport}>
          Get support…
        </MenubarItem>
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
