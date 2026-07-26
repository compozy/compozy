import "./global.css";
import { Inter, JetBrains_Mono, Playfair_Display } from "next/font/google";
import type { ReactNode } from "react";
import type { Metadata, Viewport } from "next";
import { Analytics } from "@vercel/analytics/next";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { RootProvider } from "fumadocs-ui/provider/next";
import { UIProvider } from "@agh/ui";
import { SiteFooter } from "@/components/site/site-footer";
import { SiteSearchDialog, SiteSearchProvider } from "@/components/site/site-search";
import { siteConfig } from "@/lib/site-config";

const inter = Inter({
  subsets: ["latin"],
  variable: "--font-inter",
  display: "swap",
  // Optical size axis — matches runtime Inter Variable opsz + OpenDesign.
  axes: ["opsz"],
});

const playfairDisplay = Playfair_Display({
  subsets: ["latin"],
  variable: "--font-playfair",
  display: "block",
  weight: ["400", "500"],
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL(siteConfig.url),
  applicationName: siteConfig.name,
  title: {
    default: siteConfig.name,
    template: "%s | AGH",
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
        alt: "AGH operator runtime and coordination protocol",
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
  themeColor: "#E8572A",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html
      lang="en"
      className={`dark ${inter.variable} ${playfairDisplay.variable} ${jetbrainsMono.variable}`}
    >
      <body className="flex min-h-dvh flex-col bg-fd-background font-sans text-fd-foreground antialiased">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-50 focus:rounded-lg focus:border focus:border-line focus:bg-elevated focus:px-4 focus:py-2 focus:text-sm focus:font-medium focus:text-fg"
        >
          Skip to content
        </a>
        <UIProvider>
          <SiteSearchProvider>
            <RootProvider
              search={{
                SearchDialog: SiteSearchDialog,
                options: {
                  api: "/api/search",
                },
              }}
              theme={{ enabled: false }}
            >
              {children}
              <SiteFooter />
            </RootProvider>
          </SiteSearchProvider>
        </UIProvider>
        <Analytics />
        <SpeedInsights />
      </body>
    </html>
  );
}
