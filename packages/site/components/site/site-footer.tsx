import { Eyebrow, Logo, cn } from "@agh/ui";
import { GithubLogo } from "@agh/ui/logos";
import Link from "next/link";

import { SectionFrame } from "@/components/landing/primitives/section-frame";
import { type FooterColumn, type FooterLink, footerColumns } from "@/lib/footer-config";
import { siteConfig } from "@/lib/site-config";

const LINK_CLASS =
  "inline-flex items-center text-sm text-muted transition-colors hover:text-fg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:rounded-sm";

const COLUMN_TITLE_CLASS = "eyebrow text-muted";

function FooterLinkItem({ item }: { item: FooterLink }) {
  if (item.external) {
    return (
      <a href={item.href} target="_blank" rel="noreferrer noopener" className={LINK_CLASS}>
        {item.label}
      </a>
    );
  }

  return (
    <Link href={item.href} className={LINK_CLASS}>
      {item.label}
    </Link>
  );
}

function FooterColumnGroup({ column, className }: { column: FooterColumn; className?: string }) {
  return (
    <nav aria-label={column.title} className={cn("flex flex-col gap-4", className)}>
      <h2 className={COLUMN_TITLE_CLASS}>{column.title}</h2>
      <ul className="flex flex-col gap-2.5">
        {column.items.map(item => (
          <li key={`${column.title}-${item.label}`}>
            <FooterLinkItem item={item} />
          </li>
        ))}
      </ul>
    </nav>
  );
}

export function SiteFooter() {
  const currentYear = new Date().getFullYear();
  const [runtime, network, resources] = footerColumns;

  return (
    <footer className="mt-auto border-t border-line bg-canvas">
      <SectionFrame padY="lg">
        <div className="grid grid-cols-1 gap-10 md:grid-cols-2 md:gap-12 lg:grid-cols-12">
          <div className="flex flex-col gap-5 md:col-span-2 lg:col-span-5">
            <Link href="/" aria-label="AGH home" className="inline-flex w-fit">
              <Logo variant="logo" decorative className="h-8 w-auto" />
            </Link>
            <p className="max-w-[44ch] text-sm leading-relaxed text-muted">
              {siteConfig.description}
            </p>
            <a
              href={siteConfig.githubUrl}
              target="_blank"
              rel="noreferrer noopener"
              aria-label="AGH on GitHub"
              className={cn(
                "inline-flex size-9 items-center justify-center rounded-full border border-line text-muted transition-colors",
                "hover:border-muted hover:text-fg",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
              )}
            >
              <GithubLogo aria-hidden className="size-4" />
            </a>
          </div>

          <FooterColumnGroup column={runtime} className="lg:col-span-2" />
          <FooterColumnGroup column={network} className="lg:col-span-2" />
          <FooterColumnGroup column={resources} className="lg:col-span-3" />
        </div>

        <div
          className={cn(
            "mt-14 flex flex-col gap-3 border-t border-line pt-6",
            "md:flex-row md:items-center md:justify-between"
          )}
        >
          <p className="text-xs text-subtle">
            © {currentYear} {siteConfig.name} · Built by Compozy.
          </p>
          <a
            href={siteConfig.githubUrl}
            target="_blank"
            rel="noreferrer noopener"
            className={cn(
              "inline-flex items-center gap-2 text-subtle transition-colors",
              "hover:text-fg",
              "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50 focus-visible:rounded-sm"
            )}
          >
            <Eyebrow>Alpha · Compozy on GitHub</Eyebrow>
            <span aria-hidden="true">→</span>
          </a>
        </div>
      </SectionFrame>
    </footer>
  );
}
