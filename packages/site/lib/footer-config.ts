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
    title: "Runtime",
    items: [
      { label: "Overview", href: "/runtime" },
      { label: "Getting started", href: "/runtime/core/getting-started/installation" },
      { label: "API reference", href: "/runtime/api-reference" },
      { label: "CLI reference", href: "/runtime/cli-reference" },
    ],
  },
  {
    title: "AGH Network",
    items: [
      { label: "Overview", href: "/protocol" },
      { label: "Envelope", href: "/protocol/envelope" },
      { label: "Peer discovery", href: "/protocol/peer-discovery" },
      { label: "Conformance", href: "/protocol/conformance" },
    ],
  },
  {
    title: "Resources",
    items: [
      { label: "Blog", href: "/blog" },
      { label: "Changelog", href: "/changelog" },
      { label: "GitHub", href: siteConfig.githubUrl, external: true },
      { label: "RSS", href: "/blog/feed.xml" },
    ],
  },
];
