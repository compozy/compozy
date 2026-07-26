import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";
import { Eyebrow, Logo } from "@agh/ui";
import { siteConfig } from "./site-config";

export const baseOptions: BaseLayoutProps = {
  nav: {
    title: (
      <>
        <span className="sr-only">AGH home</span>
        <Logo variant="logo" decorative className="h-8 w-auto" />
        <span aria-hidden className="hidden sm:inline-flex">
          <Eyebrow className="flex items-center gap-1.5 text-muted">
            <span className="h-1.5 w-1.5 rounded-full bg-accent" />
            Alpha
          </Eyebrow>
        </span>
      </>
    ),
    url: "/",
  },
  githubUrl: siteConfig.githubUrl,
  themeSwitch: { enabled: false },
  links: [
    { text: "Home", url: "/", active: "url" },
    { text: "Runtime", url: "/runtime", active: "nested-url" },
    { text: "AGH Network", url: "/protocol", active: "nested-url" },
    { text: "Blog", url: "/blog", active: "nested-url" },
    { text: "Changelog", url: "/changelog", active: "nested-url" },
  ],
};
