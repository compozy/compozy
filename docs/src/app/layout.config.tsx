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
      <div className="mb-1">
        <Logo size="sm" />
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
