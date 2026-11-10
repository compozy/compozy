import "./global.css";
import { Figtree, Geist_Mono, Instrument_Sans } from "next/font/google";
import type { CSSProperties, ReactNode } from "react";
import type { Metadata, Viewport } from "next";
import { Analytics } from "@vercel/analytics/next";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { RootProvider } from "fumadocs-ui/provider/next";
import { Toaster, UIProvider } from "@compozy/ui";
import { siteConfig } from "@/lib/site-config";

const SITE_TOASTER_STYLE = {
  "--normal-bg": "var(--color-canvas-soft)",
  "--normal-border": "var(--color-line-soft)",
} as CSSProperties;

const figtree = Figtree({
  subsets: ["latin"],
  variable: "--font-figtree",
  display: "swap",
});

const instrumentSans = Instrument_Sans({
  subsets: ["latin"],
  variable: "--font-instrument-sans",
  display: "swap",
  weight: ["400"],
});

const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-geist-mono",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL(siteConfig.url),
  applicationName: siteConfig.name,
  title: {
    default: siteConfig.name,
    template: "%s | CompozyOS",
  },
  description: siteConfig.description,
  alternates: {
    canonical: "/",
  },
  openGraph: {
    type: "website",
    locale: "en_US",
    url: siteConfig.url,
    siteName: siteConfig.name,
    title: siteConfig.name,
    description: siteConfig.description,
    images: [
      {
        url: "/opengraph-image",
        width: 1200,
        height: 630,
        alt: "CompozyOS: one complete environment to create, automate, and supervise agent work",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: siteConfig.name,
    description: siteConfig.description,
    images: ["/opengraph-image"],
  },
  robots: {
    index: true,
    follow: true,
  },
  icons: {
    icon: [
      { url: "/favicon.svg", type: "image/svg+xml" },
      { url: "/favicon.ico", sizes: "any" },
    ],
    apple: [{ url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" }],
  },
  manifest: "/site.webmanifest",
};

export const viewport: Viewport = {
  themeColor: "#0a0a0a",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html
      lang="en"
      className={`dark ${figtree.variable} ${instrumentSans.variable} ${geistMono.variable}`}
    >
      <body className="flex min-h-dvh flex-col bg-fd-background font-sans text-fd-foreground antialiased">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-50 focus:rounded-lg focus:border focus:border-line focus:bg-elevated focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:text-fg"
        >
          Skip to content
        </a>
        <UIProvider>
          <Toaster
            // D9: the site's toasts keep the lifted canvas-soft surface and soft hairline.
            style={SITE_TOASTER_STYLE}
          />
          {/* Docs search is the docs shell's own palette (components/docs/docs-search.tsx), a
              client of the same `/api/search` index; Fumadocs' dialog and its ⌘K are off. */}
          <RootProvider search={{ enabled: false }} theme={{ enabled: false }}>
            {children}
          </RootProvider>
        </UIProvider>
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  );
}
