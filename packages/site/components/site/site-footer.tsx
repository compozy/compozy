/** The one-line marketing footer from the landing draft. */
import { Logo, cn } from "@compozy/ui";
import Link from "next/link";

import { CONTAINER, FOCUS, HAIRLINE } from "@/components/site/layout";
import { siteConfig } from "@/lib/site-config";
import { MARKETING_NAV_LINKS } from "@/lib/site-nav";

const LINK = cn(
  "rounded-[4px] text-[14px] text-muted-foreground transition-colors hover:text-fg",
  FOCUS
);

export function SiteFooter() {
  return (
    <footer className={cn("border-t", HAIRLINE)}>
      <div
        className={cn(
          CONTAINER,
          "flex flex-col gap-6 py-9 md:flex-row md:items-center md:justify-between"
        )}
      >
        <Link
          href="/"
          aria-label="CompozyOS home"
          className={cn("flex self-start rounded-[8px] md:self-auto", FOCUS)}
        >
          <Logo variant="logo" decorative className="h-[22px]" />
        </Link>
        <nav aria-label="Footer" className="flex flex-wrap gap-x-7 gap-y-3">
          {MARKETING_NAV_LINKS.map(link => (
            <Link key={link.href} href={link.href} className={LINK}>
              {link.label}
            </Link>
          ))}
          <a href={siteConfig.githubUrl} target="_blank" rel="noopener noreferrer" className={LINK}>
            GitHub
          </a>
        </nav>
        <p className="m-0 text-[14px] text-muted-foreground">Keep every agent running.</p>
      </div>
    </footer>
  );
}
