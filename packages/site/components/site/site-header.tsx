"use client";

/** The floating marketing header from the landing draft: it compacts on scroll and draws the reading progress. */
import { Logo, cn } from "@compozy/ui";
import { Star, X } from "@compozy/ui/icons";
import { Menu } from "lucide-react";
import { m } from "motion/react";
import Link from "next/link";
import { useId } from "react";

import { useCompactHeader } from "@/components/site/hooks/use-compact-header";
import { useSiteMenu } from "@/components/site/hooks/use-site-menu";
import { STAR_FORMAT, useGithubStarCount } from "@/components/site/hooks/use-github-star-count";
import { BEAM_X, FOCUS } from "@/components/site/layout";
import { siteConfig } from "@/lib/site-config";
import { MARKETING_NAV_LINKS, isCurrentSection } from "@/lib/site-nav";

function StarButton() {
  const stars = useGithubStarCount();
  return (
    <a
      href={siteConfig.repoUrl}
      target="_blank"
      rel="noopener noreferrer"
      className={cn(
        "ml-auto inline-flex h-9 shrink-0 items-center gap-1.5 rounded-[11px] border border-site-light-150 bg-site-light-050 px-3.5 text-[13.5px] text-fg transition-colors hover:bg-site-light-100 md:ml-0",
        FOCUS
      )}
    >
      {/* On the narrowest phones only the star shows, so the menu button is never clipped. */}
      <Star className="size-3.5" aria-hidden />{" "}
      <span className="max-[379px]:sr-only">Star on GitHub</span>
      {stars === null ? null : (
        <span className="ml-2 hidden border-l border-site-light-150 pl-3 font-mono text-[12.5px] text-muted-foreground tabular-nums sm:inline">
          {STAR_FORMAT.format(stars)}
        </span>
      )}
    </a>
  );
}

function NavLink({
  href,
  label,
  current,
  onNavigate,
  className,
}: {
  href: string;
  label: string;
  current: boolean;
  onNavigate?: () => void;
  className?: string;
}) {
  return (
    <Link
      href={href}
      aria-current={current ? "page" : undefined}
      onClick={onNavigate}
      className={cn(
        "relative flex items-center rounded-[6px] text-[14px] transition-colors",
        current ? "text-fg" : "text-fg/70 hover:text-fg",
        FOCUS,
        className
      )}
    >
      {label}
      {current ? (
        <span
          aria-hidden
          className={cn("absolute inset-x-0 bottom-0 h-0.5 rounded-full", BEAM_X)}
        />
      ) : null}
    </Link>
  );
}

export function SiteHeader() {
  const { compact, progress } = useCompactHeader();
  const { pathname, open, buttonRef, close, toggle } = useSiteMenu();
  const panelId = useId();
  const solid = compact || open;
  return (
    <header className="fixed inset-x-0 top-0 z-50 flex flex-col items-center px-4 pt-3">
      <m.div
        className={cn(
          "relative flex h-14 w-full items-center justify-between gap-3 overflow-hidden rounded-[18px] border pr-2.5 pl-5 transition-[max-width,background-color,border-color,backdrop-filter] duration-500",
          solid
            ? "max-w-[920px] border-site-light-120 bg-[#101010]/75 backdrop-blur-2xl"
            : "max-w-[1280px] border-transparent bg-transparent"
        )}
      >
        <Link
          href="/"
          aria-label="CompozyOS home"
          onClick={close}
          className={cn("flex shrink-0 rounded-[8px]", FOCUS)}
        >
          <Logo variant="logo" decorative className="h-[22px]" />
        </Link>
        <nav aria-label="Site" className="hidden gap-7 self-stretch md:flex">
          {MARKETING_NAV_LINKS.map(link => (
            <NavLink
              key={link.href}
              href={link.href}
              label={link.label}
              current={isCurrentSection(pathname, link.href)}
            />
          ))}
        </nav>
        <StarButton />
        <button
          ref={buttonRef}
          type="button"
          aria-label={open ? "Close menu" : "Open menu"}
          aria-expanded={open}
          aria-controls={panelId}
          onClick={toggle}
          className={cn(
            "inline-flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-[10px] border border-site-light-120 bg-transparent text-fg/80 transition-colors hover:text-fg md:hidden",
            FOCUS
          )}
        >
          {open ? <X className="size-4" aria-hidden /> : <Menu className="size-4" aria-hidden />}
        </button>
        <m.span
          aria-hidden
          style={{ scaleX: progress }}
          className={cn(
            "absolute inset-x-0 bottom-0 h-px origin-left bg-[linear-gradient(90deg,#1f87fc,#1df0d7)] transition-opacity",
            compact ? "opacity-100" : "opacity-0"
          )}
        />
      </m.div>
      <nav
        id={panelId}
        aria-label="Site menu"
        hidden={!open}
        className="mt-2 w-full max-w-[920px] rounded-[18px] border border-site-light-120 bg-[#101010]/95 p-2 backdrop-blur-2xl md:hidden"
      >
        <ul className="m-0 flex list-none flex-col p-0">
          {MARKETING_NAV_LINKS.map(link => (
            <li key={link.href} className="border-b border-site-light-080 last:border-b-0">
              <NavLink
                href={link.href}
                label={link.label}
                current={isCurrentSection(pathname, link.href)}
                onNavigate={close}
                className="h-12 px-3 text-[15px]"
              />
            </li>
          ))}
        </ul>
      </nav>
    </header>
  );
}
