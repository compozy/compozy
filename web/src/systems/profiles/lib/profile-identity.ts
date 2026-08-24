import {
  Anchor,
  Award,
  Bike,
  Book,
  Briefcase,
  Brush,
  Camera,
  Cloud,
  Coffee,
  Compass,
  Cpu,
  Crown,
  Flame,
  Gem,
  Globe,
  GraduationCap,
  Heart,
  Leaf,
  Lightbulb,
  Map,
  Megaphone,
  Mic,
  Moon,
  Music,
  Palette,
  PenTool,
  Pencil,
  Rocket,
  Shield,
  Sparkles,
  Star,
  Sun,
  Target,
  TrendingUp,
  UserRound,
  Users,
  Wrench,
  Zap,
} from "lucide-react";

import type {
  KindIconRegistry,
  SymbolEmojiOption,
  SymbolIconOption,
  SymbolSwatch,
  SymbolValue,
} from "@compozy/ui";

import type { ProfilePayload } from "../types";

/**
 * The bundled symbol set.
 *
 * Extends the `KindIcon` registry pattern rather than inventing a second icon
 * mechanism: the picker renders through `KindIcon` with this registry, so a
 * profile's stored `icon` token is resolved exactly like every other kind glyph.
 */
export const PROFILE_ICON_REGISTRY = {
  "user-round": UserRound,
  megaphone: Megaphone,
  briefcase: Briefcase,
  rocket: Rocket,
  palette: Palette,
  camera: Camera,
  music: Music,
  heart: Heart,
  star: Star,
  zap: Zap,
  globe: Globe,
  book: Book,
  coffee: Coffee,
  flame: Flame,
  leaf: Leaf,
  cloud: Cloud,
  moon: Moon,
  sun: Sun,
  anchor: Anchor,
  award: Award,
  bike: Bike,
  brush: Brush,
  compass: Compass,
  cpu: Cpu,
  crown: Crown,
  gem: Gem,
  "graduation-cap": GraduationCap,
  "trending-up": TrendingUp,
  wrench: Wrench,
  target: Target,
  lightbulb: Lightbulb,
  map: Map,
  mic: Mic,
  "pen-tool": PenTool,
  pencil: Pencil,
  shield: Shield,
  sparkles: Sparkles,
  users: Users,
} satisfies KindIconRegistry;

export const PROFILE_ICON_OPTIONS: readonly SymbolIconOption[] = Object.keys(
  PROFILE_ICON_REGISTRY
).map(name => ({ name, label: name.replace(/-/g, " ") }));

export const PROFILE_EMOJI_OPTIONS: readonly SymbolEmojiOption[] = [
  { value: "🚀", label: "rocket", keywords: "launch ship" },
  { value: "🌱", label: "seedling", keywords: "growth plant" },
  { value: "🎨", label: "artist palette", keywords: "art design" },
  { value: "📣", label: "megaphone", keywords: "marketing announce" },
  { value: "💼", label: "briefcase", keywords: "work business" },
  { value: "📈", label: "chart increasing", keywords: "growth revenue" },
  { value: "✍️", label: "writing hand", keywords: "write copy" },
  { value: "☕", label: "hot beverage", keywords: "coffee break" },
  { value: "🎯", label: "direct hit", keywords: "goal focus target" },
  { value: "🧪", label: "test tube", keywords: "experiment lab" },
  { value: "📚", label: "books", keywords: "docs reading" },
  { value: "🛠️", label: "hammer and wrench", keywords: "tools build" },
  { value: "🧭", label: "compass", keywords: "explore direction" },
  { value: "🏔️", label: "mountain", keywords: "peak climb" },
  { value: "🌊", label: "water wave", keywords: "ocean flow" },
  { value: "🔬", label: "microscope", keywords: "research science" },
  { value: "🎬", label: "clapper board", keywords: "video film" },
  { value: "🧵", label: "thread", keywords: "sew craft" },
];

/**
 * Suggested colors, deliberately clear of the signal hexes so an identity can
 * never be mistaken for a status. A typed value may still land on one — that is
 * the operator's data, and the picker accepts it.
 */
export const PROFILE_IDENTITY_SWATCHES: readonly SymbolSwatch[] = [
  { label: "Gray", value: "#8a8f98" },
  { label: "Blue", value: "#4ea7fc" },
  { label: "Teal", value: "#2aa8a8" },
  { label: "Green", value: "#4cb782" },
  { label: "Violet", value: "#c26ad6" },
  { label: "Pink", value: "#e56aa2" },
  { label: "Amber", value: "#e8b04a" },
  { label: "Brown", value: "#b58e5f" },
];

const STARTER_ICONS = [
  "compass",
  "rocket",
  "leaf",
  "sparkles",
  "target",
  "flame",
  "gem",
  "anchor",
] as const;

/**
 * A profile is never born blank (US-001.AC-3): creation pre-picks a distinct
 * pairing so it is recognisable from its first second. The pick rotates off the
 * count of profiles that already exist, so two profiles made back to back do not
 * look alike.
 */
export function starterIdentity(existingCount: number): { color: string; icon: string } {
  const index = Math.max(0, existingCount);
  const swatch = PROFILE_IDENTITY_SWATCHES[index % PROFILE_IDENTITY_SWATCHES.length];
  return {
    color: swatch.value,
    icon: STARTER_ICONS[index % STARTER_ICONS.length],
  };
}

/** The symbol a stored profile renders, normalized for the picker. */
export function symbolOf(profile: Pick<ProfilePayload, "icon" | "emoji">): SymbolValue {
  const emoji = profile.emoji?.trim();
  if (emoji) return { kind: "emoji", value: emoji };
  const icon = profile.icon?.trim();
  return { kind: "icon", value: icon && icon !== "" ? icon : "user-round" };
}

/** Turns a picked symbol back into the mutually exclusive wire fields. */
export function symbolPatch(symbol: SymbolValue): { icon?: string; emoji?: string } {
  return symbol.kind === "emoji" ? { emoji: symbol.value } : { icon: symbol.value };
}
