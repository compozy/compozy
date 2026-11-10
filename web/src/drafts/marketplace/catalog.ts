/** A real snapshot of compozy/plugins plus the extensions feed; categories follow the daemon's fixed order. */
import {
  Blocks,
  Bot,
  ChartBar,
  Code2,
  Coins,
  Compass,
  Globe,
  Megaphone,
  MessagesSquare,
  Palette,
  Search,
  Shield,
  SquareCheck,
} from "@compozy/ui/icons";

import raw from "./catalog.json";

export interface Entry {
  id: string;
  name: string;
  author: string | null;
  category: string;
  description: string;
  icon: string | null;
  featured: boolean;
  homepage: string | null;
  kind: "plugin" | "extension";
  source: string;
  tier: string;
  bleed: boolean;
  light: boolean;
}

export const ENTRIES = raw as Entry[];

export const CATEGORIES = [
  { id: "productivity", label: "Productivity", icon: SquareCheck },
  { id: "communication", label: "Communication", icon: MessagesSquare },
  { id: "design", label: "Design", icon: Palette },
  { id: "development", label: "Development", icon: Code2 },
  { id: "data", label: "Data & Analytics", icon: ChartBar },
  { id: "cloud", label: "Cloud & Infrastructure", icon: Globe },
  { id: "finance", label: "Finance & Payments", icon: Coins },
  { id: "sales-marketing", label: "Sales & Marketing", icon: Megaphone },
  { id: "research", label: "Research & Learning", icon: Search },
  { id: "security", label: "Security", icon: Shield },
  { id: "ai-agents", label: "AI & Agents", icon: Bot },
  { id: "more", label: "More", icon: Blocks },
] as const;

export const ALL_ICON = Compass;

export function categoryLabel(id: string) {
  return CATEGORIES.find(category => category.id === id)?.label ?? "More";
}

export function inCategory(id: string) {
  return ENTRIES.filter(entry => entry.category === id);
}

export function findEntry(id: string) {
  return ENTRIES.find(entry => entry.id === id);
}

/** Feed descriptions run long; a card only needs the first sentence. */
export function firstSentence(text: string) {
  const match = /^.+?[.!?](?=\s|$)/.exec(text);
  return match ? match[0] : text;
}

export function matches(entry: Entry, needle: string) {
  return `${entry.name} ${entry.author ?? ""} ${entry.description} ${entry.id}`
    .toLowerCase()
    .includes(needle);
}

export const FEATURED = ENTRIES.filter(entry => entry.featured);
