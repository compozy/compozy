import { RootProvider } from "fumadocs-ui/provider";
import { GeistMono } from "geist/font/mono";
import { GeistSans } from "geist/font/sans";
import type { Metadata } from "next";
import localFont from "next/font/local";
import type { ReactNode } from "react";
import "./global.css";

// Load Clash Display Variable font (single file with all weights 200-700)
const clashDisplay = localFont({
  src: "../../public/fonts/ClashDisplay-Variable.woff2",
  variable: "--font-clash-display",
  display: "swap",
  weight: "200 700", // Variable font weight range
});

export const metadata: Metadata = {
  title: "Compozy",
  description: "Next-level AI-agentic orchestraion platform",
  icons: {
    icon: "/icon",
    apple: "/apple-icon",
  },
};

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <html
      lang="en"
      className={`${GeistSans.variable} ${GeistMono.variable} ${clashDisplay.variable}`}
      suppressHydrationWarning
    >
      <body className="flex flex-col min-h-screen font-sans">
        <RootProvider>{children}</RootProvider>
      </body>
    </html>
  );
}
