import { Badge } from "@/components/ui/badge";
import { Logo } from "@/components/ui/logo";
import type { BaseLayoutProps } from "fumadocs-ui/layouts/shared";

/**
 * Shared layout configurations
 *
 * you can customise layouts individually from:
 * Home Layout: app/(home)/layout.tsx
 * Docs Layout: app/docs/layout.tsx
 */
export const baseOptions: BaseLayoutProps = {
  disableThemeSwitch: true,
  themeSwitch: {
    enabled: false,
  },
  nav: {
    title: (
      <div className="mb-1 flex items-center gap-2">
        <Logo size="sm" />
        <Badge variant="secondary" size="sm" className="text-[10px] px-1.5 py-0">
          ALPHA
        </Badge>
      </div>
    ),
  },
  githubUrl: "https://github.com/compozy/compozy",
  // see https://fumadocs.dev/docs/ui/navigation/links
  links: [
    {
      text: "Docs",
      url: "/docs",
      active: "nested-url",
    },
    {
      text: "Install",
      url: "/docs/core/getting-started/installation",
      active: "nested-url",
    },
    {
      text: "Getting Started",
      url: "/docs/core/getting-started/quick-start",
      active: "nested-url",
    },
    {
      text: "Features",
      url: "/#features",
    },
    {
      text: "Pricing",
      url: "/#pricing",
    },
  ],
};
