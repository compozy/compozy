import {
  DiscordLogo,
  GithubLogo,
  GoogleChatLogo,
  LinearLogo,
  MicrosoftTeamsLogo,
  SlackLogo,
  TelegramLogo,
  WhatsAppLogo,
} from "@compozy/ui/logos";
import type { ReactNode } from "react";

/**
 * Real platform marks from the `@compozy/ui` logo inventory — the same components the landing
 * page's bridges section renders. Data, not a component, so the bridge grid and the coverage test
 * read one source: a provider whose manifest lands without a mark here is caught by
 * `lib/__tests__/marketplace-catalog.test.tsx` rather than silently rendering a neutral glyph.
 */
export const BRIDGE_LOGOS: Record<string, ReactNode> = {
  slack: <SlackLogo aria-hidden className="size-5" />,
  discord: <DiscordLogo aria-hidden className="size-5" />,
  telegram: <TelegramLogo aria-hidden className="size-5" />,
  whatsapp: <WhatsAppLogo aria-hidden className="size-5" />,
  teams: <MicrosoftTeamsLogo aria-hidden className="size-5" />,
  gchat: <GoogleChatLogo aria-hidden className="size-5" />,
  github: <GithubLogo aria-hidden className="size-5 text-fg" />,
  linear: <LinearLogo aria-hidden className="size-5" mode="dark" />,
};
