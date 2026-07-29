import { siteConfig } from "./site-config";

export type FooterLink = {
  label: string;
  href: string;
  external?: boolean;
};

export type FooterColumn = {
  title: string;
  items: FooterLink[];
};

export const footerColumns: FooterColumn[] = [
  {
    title: "CompozyOS",
    items: [
      { label: "Overview", href: "/docs" },
      { label: "Getting started", href: "/docs/getting-started/installation" },
      { label: "Migrate from v0.2", href: "/docs/migration" },
      { label: "API reference", href: "/docs/api" },
      { label: "CLI reference", href: "/docs/cli" },
    ],
  },
  {
    title: "Compozy Network",
    items: [
      { label: "Overview", href: "/docs/network" },
      { label: "Protocol spec", href: "/docs/network/protocol" },
      { label: "Envelope", href: "/docs/network/protocol/envelope" },
      { label: "Conformance", href: "/docs/network/protocol/conformance" },
    ],
  },
  {
    title: "Resources",
    items: [
      { label: "Marketplace", href: "/marketplace" },
      { label: "Blog", href: "/blog" },
      { label: "Changelog", href: "/changelog" },
      { label: "GitHub", href: siteConfig.githubUrl, external: true },
      { label: "RSS", href: "/blog/feed.xml" },
    ],
  },
];
