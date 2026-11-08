// The kit registry: every template id in ../kit-specs.ts (size, margins, print) mapped to its
// React component and the field values that make a variant. /kit/$id, /kit/index.json, the
// chapter <Frame> previews and `export:kit` all read from here.
import { kitSpecs, type KitSpec } from "../kit-specs";
import {
  AdDevBanner,
  AdLandscape,
  AdLinkedin,
  AdPortrait,
  AdSquareBrand,
  AdSquareProof,
  AdStory,
  AdXCard,
} from "./ads/ads";
import { BlogHero, CallBackground, CodeCard, NewsletterHeader, Wallpaper } from "./editorial/media";
import { OgBlog, OgChangelog, OgDefault, OgDocs } from "./editorial/og";
import {
  SlideCode,
  SlideContent,
  SlideDiagram,
  SlideSection,
  SlideTitle,
} from "./editorial/slides";
import { EventBadge } from "./offline/badge";
import { Poster, Rollup, StickerSheet, TeeChest } from "./offline/print";
import {
  BusinessCardBack,
  BusinessCardFront,
  EmailSignature,
  Envelope,
  Letterhead,
} from "./offline/stationery";
import { DesktopWallpaper } from "./product/desktop-wallpaper";
import { CarouselBody, CarouselCover, CarouselCta } from "./social/carousel";
import {
  GithubPreview,
  LinkedinPost,
  ReadmeHeader,
  ReleaseCard,
  Story,
  XPost,
  XQuote,
  YoutubeThumb,
} from "./social/posts";
import {
  Avatar,
  DiscordBanner,
  LinkedinBanner,
  LinkedinCover,
  XHeader,
  YoutubeBanner,
} from "./social/profile";
import type { KitFields, KitTemplate } from "./types";

const t = (component: KitTemplate["component"], fields?: Record<string, string>): KitTemplate => ({
  component,
  fields,
});
const badge = (variant: string, role: string, company: string) =>
  t(EventBadge, { variant, role, name: "Name Surname", company });

const TEMPLATES: Record<string, KitTemplate> = {
  "x-post": t(XPost),
  "x-quote": t(XQuote),
  "x-header": t(XHeader),
  "x-avatar": t(Avatar),
  "linkedin-banner": t(LinkedinBanner),
  "linkedin-cover": t(LinkedinCover),
  "linkedin-avatar": t(Avatar),
  "linkedin-post": t(LinkedinPost),
  "carousel-cover": t(CarouselCover),
  "carousel-body": t(CarouselBody),
  "carousel-cta": t(CarouselCta),
  story: t(Story),
  "youtube-banner": t(YoutubeBanner),
  "youtube-avatar": t(Avatar),
  "youtube-thumb": t(YoutubeThumb),
  "github-social-preview": t(GithubPreview),
  "github-avatar": t(Avatar),
  "readme-header": t(ReadmeHeader),
  "discord-banner": t(DiscordBanner),
  "discord-icon": t(Avatar),
  "release-card": t(ReleaseCard),
  "ad-square-brand": t(AdSquareBrand),
  "ad-square-proof": t(AdSquareProof),
  "ad-portrait": t(AdPortrait),
  "ad-story": t(AdStory),
  "ad-landscape": t(AdLandscape),
  "ad-x-card": t(AdXCard),
  "ad-linkedin": t(AdLinkedin),
  "ad-dev-banner": t(AdDevBanner),
  "og-default": t(OgDefault),
  "og-blog": t(OgBlog),
  "og-docs": t(OgDocs),
  "og-changelog": t(OgChangelog),
  "blog-hero": t(BlogHero),
  "newsletter-header": t(NewsletterHeader),
  "slide-title": t(SlideTitle),
  "slide-section": t(SlideSection),
  "slide-content": t(SlideContent),
  "slide-code": t(SlideCode),
  "slide-diagram": t(SlideDiagram),
  "code-card": t(CodeCard),
  "call-background": t(CallBackground),
  wallpaper: t(Wallpaper),
  "business-card-front": t(BusinessCardFront),
  "business-card-back": t(BusinessCardBack),
  letterhead: t(Letterhead),
  "envelope-dl": t(Envelope),
  "badge-team": badge("team", "Team", "Compozy"),
  "badge-speaker": badge("speaker", "Speaker", "Talk title in one line"),
  "badge-partner": badge("partner", "Partner", "Partner company"),
  "badge-attendee": badge("attendee", "Attendee", "Company"),
  "sticker-sheet": t(StickerSheet),
  "poster-a3": t(Poster),
  rollup: t(Rollup),
  "tee-chest": t(TeeChest),
  "email-signature": t(EmailSignature),
  "desktop-wallpaper": t(DesktopWallpaper),
};

/** Every kit id that has a component, in kit-spec order. */
export const kitIds = (): string[] => kitSpecs.map(s => s.id).filter(id => id in TEMPLATES);

/** Ids registered as components (the registry test checks both lists match). */
export const templateIds = (): string[] => Object.keys(TEMPLATES);

export interface ResolvedTemplate {
  spec: KitSpec;
  template: KitTemplate;
}

/** A kit id → its spec and component, or null when the id is unknown. */
export function findTemplate(id: string): ResolvedTemplate | null {
  const spec = kitSpecs.find(s => s.id === id);
  const template = TEMPLATES[id];
  return spec && template ? { spec, template } : null;
}

/** Template defaults overlaid with caller fields (empty values ignored). */
export function fieldsOf(template: KitTemplate, overrides: KitFields = {}): KitFields {
  const set = Object.fromEntries(
    Object.entries(overrides).filter(([, v]) => v !== undefined && v !== "")
  );
  return { ...template.fields, ...set };
}

/** /kit/index.json rows: `[{ id, w, h, group }]` for every template (_dx.md). */
export const kitIndex = () =>
  kitSpecs
    .filter(s => s.id in TEMPLATES)
    .map(s => ({ id: s.id, w: s.w, h: s.h, group: s.channel }));
