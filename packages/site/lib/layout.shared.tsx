import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { Eyebrow, Logo } from "@compozy/ui";
import { siteConfig } from "./site-config";
import { SITE_NAV_LINKS } from "./site-nav";

export const baseOptions: BaseLayoutProps = {
  nav: {
    title: (
      <>
        <span className="sr-only">CompozyOS home</span>
        <Logo variant="logo" decorative className="h-8 w-auto" />
        <span aria-hidden className="hidden sm:inline-flex">
          <Eyebrow className="flex items-center gap-1.5 text-muted">
            <span className="h-1.5 w-1.5 rounded-full bg-accent" />
            Beta
          </Eyebrow>
        </span>
      </>
    ),
    url: "/",
  },
  githubUrl: siteConfig.githubUrl,
  themeSwitch: { enabled: false },
  links: SITE_NAV_LINKS.map(link => ({
    text: link.label,
    url: link.href,
    active: link.active,
  })),
};
